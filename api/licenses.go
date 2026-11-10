package api

import (
	"context"
	"fmt"
	"strings"
)

type LicenseAttribute struct {
	Name  string
	Value string
}

type License struct {
	ID                 string             `graphql:"id"`
	Key                string             `graphql:"key"`
	LicenseVersion     string             `graphql:"licenseVersion"`
	ProductID          string             `graphql:"productId"`
	ProductDescription string             `graphql:"productDescription"`
	FoundationCount    int                `graphql:"foundationCount"`
	Expiration         string             `graphql:"expiration"`
	ExtendedAttributes []LicenseAttribute `graphql:"extendedAttributes"`
}

func (g *GraphQLClient) QueryLicenses(ctx context.Context) ([]License, error) {
	var query struct {
		LicenseQuery struct {
			QueryLicenses []License `graphql:"queryLicenses"`
		} `graphql:"licenseQuery"`
	}

	if err := g.Client.Query(ctx, &query, nil); err != nil {
		return nil, err
	}

	return query.LicenseQuery.QueryLicenses, nil
}

type LicenseAddResult struct {
	Success             bool     `graphql:"success"`
	VerificationFailure string   `graphql:"verificationFailure"`
	License             *License `graphql:"license"`
}

func (g *GraphQLClient) AddLicense(ctx context.Context, licenseKey string) (*LicenseAddResult, error) {
	var mutation struct {
		LicenseMutation struct {
			AddLicense LicenseAddResult `graphql:"addLicense(licenseKey: $licenseKey)"`
		} `graphql:"licenseMutation"`
	}

	vars := map[string]any{
		"licenseKey": licenseKey,
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return nil, err
	}

	result := mutation.LicenseMutation.AddLicense
	return &result, nil
}

func (g *GraphQLClient) DeleteLicense(ctx context.Context, licenseID string) (bool, error) {
	var mutation struct {
		LicenseMutation struct {
			DeleteLicense struct {
				Success bool `graphql:"success"`
			} `graphql:"deleteLicense(licenseId: $licenseId)"`
		} `graphql:"licenseMutation"`
	}

	vars := map[string]any{
		"licenseId": licenseID,
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return false, err
	}

	return mutation.LicenseMutation.DeleteLicense.Success, nil
}

type LicenseApplicationFoundationWorkflow struct {
	FoundationID string   `graphql:"foundationId"`
	Status       string   `graphql:"status"`
	WorkflowID   string   `graphql:"workflowId"`
	Errors       []string `graphql:"errors"`
}

type LicenseApplyResult struct {
	LicenseApplicationResults []LicenseApplicationFoundationWorkflow `graphql:"licenseApplicationResults"`
}

// FoundationEntity is a foundation as known to Hub's entity graph.
type FoundationEntity struct {
	EntityID           string `graphql:"entityId"`
	EntityName         string `graphql:"entityName"`
	ManagementEndpoint struct {
		ManagementEndpointID string `graphql:"managementEndpointId"`
		EndpointName         string `graphql:"endpointName"`
	} `graphql:"managementEndpoint"`
}

// QueryFoundationEntities lists the foundation entities known to Hub.
func (g *GraphQLClient) QueryFoundationEntities(ctx context.Context) ([]FoundationEntity, error) {
	var query struct {
		EntityQuery struct {
			QueryEntities struct {
				Entities []FoundationEntity `graphql:"entities"`
			} `graphql:"queryEntities(entityType: \"Tanzu.TAS.Foundation\", first: 1000)"`
		} `graphql:"entityQuery"`
	}

	if err := g.Client.Query(ctx, &query, nil); err != nil {
		return nil, err
	}
	return query.EntityQuery.QueryEntities.Entities, nil
}

// ResolveFoundationEntityIDs converts user supplied foundations into the
// foundation entity IDs (VRNs) Hub expects when applying a license. Each value
// may be a full entity ID (passed through unchanged), the ID shown by
// "foundation list" (the foundation's management endpoint ID), or a foundation
// name.
func (g *GraphQLClient) ResolveFoundationEntityIDs(ctx context.Context, ids []string) ([]string, error) {
	var foundations []FoundationEntity
	fetched := false
	resolved := make([]string, len(ids))
	for i, id := range ids {
		if strings.HasPrefix(id, "vrn/") {
			resolved[i] = id
			continue
		}
		if !fetched {
			var err error
			if foundations, err = g.QueryFoundationEntities(ctx); err != nil {
				return nil, err
			}
			fetched = true
		}
		entityID, err := matchFoundation(foundations, id)
		if err != nil {
			return nil, err
		}
		resolved[i] = entityID
	}
	return resolved, nil
}

func matchFoundation(foundations []FoundationEntity, id string) (string, error) {
	var matches []string
	for _, f := range foundations {
		if f.EntityName == id || f.EntityID == id ||
			f.ManagementEndpoint.ManagementEndpointID == id || f.ManagementEndpoint.EndpointName == id {
			matches = append(matches, f.EntityID)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		known := make([]string, len(foundations))
		for i, f := range foundations {
			known[i] = fmt.Sprintf("%s (%s)", f.EntityName, f.EntityID)
		}
		return "", fmt.Errorf("no foundation found matching %q; use the ID or name from 'foundation list', or a full entity ID. Known foundations: %s",
			id, strings.Join(known, ", "))
	default:
		return "", fmt.Errorf("%d foundations match %q; use the full entity ID", len(matches), id)
	}
}

func (g *GraphQLClient) ApplyLicenseToFoundations(ctx context.Context, licenseID string, foundationIDs []string) (*LicenseApplyResult, error) {
	var mutation struct {
		LicenseMutation struct {
			ApplyLicenseToFoundations LicenseApplyResult `graphql:"applyLicenseToFoundations(input: $input)"`
		} `graphql:"licenseMutation"`
	}

	type LicenseApplyToFoundationsInput struct {
		LicenseID     string   `json:"licenseId"`
		FoundationIDs []string `json:"foundationIds"`
	}

	vars := map[string]any{
		"input": LicenseApplyToFoundationsInput{
			LicenseID:     licenseID,
			FoundationIDs: foundationIDs,
		},
	}

	if err := g.Client.Mutate(ctx, &mutation, vars); err != nil {
		return nil, err
	}

	result := mutation.LicenseMutation.ApplyLicenseToFoundations
	return &result, nil
}
