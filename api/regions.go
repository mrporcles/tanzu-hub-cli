package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	graphql "github.com/hasura/go-graphql-client"
)

type RegionMember struct {
	EntityID   string `graphql:"entityId" json:"id" yaml:"id"`
	EntityName string `graphql:"entityName" json:"name" yaml:"name"`
}

type Region struct {
	EntityID    string         `json:"id" yaml:"id"`
	EntityName  string         `json:"name" yaml:"name"`
	Foundations []RegionMember `json:"foundations" yaml:"foundations"`
}

type GroupFoundationGroupCreateInput struct {
	Name string `json:"name"`
}

// CreateRegion creates a foundation group (region) and returns its ID.
func (g *GraphQLClient) CreateRegion(ctx context.Context, name string) (string, error) {
	var mutation struct {
		GroupMutation struct {
			FoundationGroupMutation struct {
				Create []struct {
					FoundationGroup struct {
						FoundationGroupID string `graphql:"foundationGroupId"`
					} `graphql:"foundationGroup"`
					Status struct {
						Status       string `graphql:"status"`
						ErrorMessage string `graphql:"errorMessage"`
					} `graphql:"status"`
				} `graphql:"create(input: $input)"`
			} `graphql:"foundationGroupMutation"`
		} `graphql:"groupMutation"`
	}

	vars := map[string]any{
		"input": []GroupFoundationGroupCreateInput{{Name: name}},
	}

	if err := g.Client.Mutate(ctx, &mutation, vars, graphql.OperationName("createFoundationGroup")); err != nil {
		return "", err
	}

	results := mutation.GroupMutation.FoundationGroupMutation.Create
	if len(results) == 0 {
		return "", fmt.Errorf("no result returned when creating region %q", name)
	}
	r := results[0]
	if r.Status.ErrorMessage != "" {
		return "", fmt.Errorf("%s: %s", r.Status.Status, r.Status.ErrorMessage)
	}
	return r.FoundationGroup.FoundationGroupID, nil
}

// QueryRegions lists foundation groups and their member foundations.
func (g *GraphQLClient) QueryRegions(ctx context.Context) ([]Region, error) {
	var query struct {
		EntityQuery struct {
			QueryEntities struct {
				Entities []struct {
					EntityID   string `graphql:"entityId"`
					EntityName string `graphql:"entityName"`
					EntitiesIn struct {
						Entities []RegionMember `graphql:"entities"`
					} `graphql:"entitiesIn(entityTypeConst: Tanzu_TAS_Foundation, first: 1000)"`
				} `graphql:"entities"`
			} `graphql:"queryEntities(entityType: \"Tanzu.Platform.FoundationGroup\", first: 1000)"`
		} `graphql:"entityQuery"`
	}

	if err := g.Client.Query(ctx, &query, nil, graphql.OperationName("getFoundationGroupsAndFoundations")); err != nil {
		return nil, err
	}

	groups := make([]Region, 0, len(query.EntityQuery.QueryEntities.Entities))
	for _, e := range query.EntityQuery.QueryEntities.Entities {
		groups = append(groups, Region{
			EntityID:    e.EntityID,
			EntityName:  e.EntityName,
			Foundations: e.EntitiesIn.Entities,
		})
	}
	return groups, nil
}

const deleteRegionMutation = `mutation deleteFoundationGroup($id: [EntityId!]!) {
  groupMutation {
    foundationGroupMutation {
      delete(foundationGroupId: $id) {
        status {
          status
          errorMessage
        }
      }
    }
  }
}`

type regionStatus struct {
	Status       string  `json:"status"`
	ErrorMessage *string `json:"errorMessage"`
}

type deleteRegionResult struct {
	Status regionStatus `json:"status"`
}

// DeleteRegion deletes the foundation group with the given ID.
func (g *GraphQLClient) DeleteRegion(ctx context.Context, id string) error {
	raw, err := g.Client.ExecRaw(ctx, deleteRegionMutation, map[string]any{"id": []string{id}}, graphql.OperationName("deleteFoundationGroup"))
	if err != nil {
		return err
	}

	var data struct {
		GroupMutation struct {
			FoundationGroupMutation struct {
				Delete json.RawMessage `json:"delete"`
			} `json:"foundationGroupMutation"`
		} `json:"groupMutation"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("decoding delete response: %w", err)
	}

	// The input is a list, so the result is likely a list, but accept a single object too.
	del := bytes.TrimSpace(data.GroupMutation.FoundationGroupMutation.Delete)
	var results []deleteRegionResult
	if len(del) > 0 && del[0] == '[' {
		if err := json.Unmarshal(del, &results); err != nil {
			return fmt.Errorf("decoding delete response: %w", err)
		}
	} else {
		var single deleteRegionResult
		if err := json.Unmarshal(del, &single); err != nil {
			return fmt.Errorf("decoding delete response: %w", err)
		}
		results = []deleteRegionResult{single}
	}

	for _, r := range results {
		if r.Status.ErrorMessage != nil && *r.Status.ErrorMessage != "" {
			return fmt.Errorf("%s: %s", r.Status.Status, *r.Status.ErrorMessage)
		}
	}
	return nil
}
