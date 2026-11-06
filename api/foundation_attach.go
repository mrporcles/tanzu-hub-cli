package api

import (
	"context"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
)

type FleetManagementFoundationSpecInput struct {
	Labels []string `json:"labels"`
}

// foundationPropertyKeys maps the deployment property names returned when
// generating a foundation's deployment properties to the output keys used by
// the legacy attach flow, so both flows produce the same output.
var foundationPropertyKeys = map[string]string{
	"hub_ca_certificate":  "caBundle",
	"foundation_id":       "foundation-id",
	"collector_id":        "collector-id",
	"ingestion_url":       "ingestion_url",
	"oauth_client_id":     "oauth-app-id",
	"oauth_client_secret": "oauth-app-secret",
	"csp_org_id":          "org-id",
}

// FindRegion returns the foundation group matching the given name, or
// the given ID if id is set. It errors unless exactly one group matches.
func (g *GraphQLClient) FindRegion(ctx context.Context, name, id string) (*Region, error) {
	groups, err := g.QueryRegions(ctx)
	if err != nil {
		return nil, fmt.Errorf("looking up region: %w", err)
	}

	var matches []Region
	for _, region := range groups {
		if (id != "" && region.EntityID == id) || (id == "" && region.EntityName == name) {
			matches = append(matches, region)
		}
	}

	switch {
	case len(matches) == 1:
		return &matches[0], nil
	case len(matches) == 0 && id != "":
		return nil, fmt.Errorf("no region found with ID %q", id)
	case len(matches) == 0:
		return nil, fmt.Errorf("no region found with name %q", name)
	default:
		return nil, fmt.Errorf("%d regions found with name %q", len(matches), name)
	}
}

// FindRegionID returns the ID of the foundation group with the given name.
func (g *GraphQLClient) FindRegionID(ctx context.Context, name string) (string, error) {
	region, err := g.FindRegion(ctx, name, "")
	if err != nil {
		return "", err
	}
	return region.EntityID, nil
}

// AttachFoundationInRegion creates a foundation in the named foundation group and
// generates its deployment properties, including a new OAuth app.
func (g *GraphQLClient) AttachFoundationInRegion(ctx context.Context, name, regionName string) (map[string]string, error) {
	regionID, err := g.FindRegionID(ctx, regionName)
	if err != nil {
		return nil, err
	}

	var create struct {
		FleetManagementMutation struct {
			CreateFoundation struct {
				FoundationEntityID string `graphql:"foundationEntityId"`
				Status             string `graphql:"status"`
				ErrorMessage       string `graphql:"errorMessage"`
			} `graphql:"createFoundation(spec: $spec, name: $name, regionEntityId: $regionEntityId)"`
		} `graphql:"fleetManagementMutation"`
	}

	createVars := map[string]any{
		"spec":           FleetManagementFoundationSpecInput{Labels: []string{}},
		"name":           name,
		"regionEntityId": EntityId(regionID),
	}
	if err := g.Client.Mutate(ctx, &create, createVars, graphql.OperationName("createFoundation")); err != nil {
		return nil, fmt.Errorf("creating foundation: %w", err)
	}

	c := create.FleetManagementMutation.CreateFoundation
	if c.ErrorMessage != "" {
		return nil, fmt.Errorf("creating foundation: %s: %s", c.Status, c.ErrorMessage)
	}
	if c.FoundationEntityID == "" {
		return nil, fmt.Errorf("creating foundation: no foundation ID returned (status %q)", c.Status)
	}

	var generate struct {
		FleetManagementMutation struct {
			GenerateFoundationDeploymentProperties struct {
				DeploymentProperties []ManagementEndpointCollectorDeploymentProperty `graphql:"deploymentProperties"`
				Foundation           struct {
					EntityID   string `graphql:"entityId"`
					EntityName string `graphql:"entityName"`
				} `graphql:"foundation"`
			} `graphql:"generateFoundationDeploymentProperties(foundationId: $foundationId, generateOAuthApp: $generateOAuthApp)"`
		} `graphql:"fleetManagementMutation"`
	}

	generateVars := map[string]any{
		"foundationId":     EntityId(c.FoundationEntityID),
		"generateOAuthApp": true,
	}
	if err := g.Client.Mutate(ctx, &generate, generateVars, graphql.OperationName("generateFoundationDeploymentProperties")); err != nil {
		return nil, fmt.Errorf("foundation %q was created (ID %s) but generating deployment properties failed: %w", name, c.FoundationEntityID, err)
	}

	r := generate.FleetManagementMutation.GenerateFoundationDeploymentProperties
	result := map[string]string{
		"id":                   c.FoundationEntityID,
		"name":                 name,
		"foundationName":       r.Foundation.EntityName,
		"region":               regionName,
		"foundation-entity-id": c.FoundationEntityID,
	}
	for _, prop := range r.DeploymentProperties {
		key, ok := foundationPropertyKeys[prop.Name]
		if !ok {
			continue
		}
		if v, ok := prop.Value.(string); ok {
			result[key] = v
		}
	}
	return result, nil
}
