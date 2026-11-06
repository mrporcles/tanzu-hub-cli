package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gqlServer(t *testing.T, response string, gotBody *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		b, _ := io.ReadAll(r.Body)
		*gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
}

func TestCreateRegion(t *testing.T) {
	var body string
	server := gqlServer(t, `{"data":{"groupMutation":{"foundationGroupMutation":{"create":[{"foundationGroup":{"foundationGroupId":"fg-1"},"status":{"status":"SUCCESS","errorMessage":null}}]}}}}`, &body)
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	id, err := NewGraphQLClient(client, server.URL, true).CreateRegion(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, "fg-1", id)
	assert.Contains(t, body, "[GroupFoundationGroupCreateInput!]!")
	assert.Contains(t, body, `"name":"test"`)
}

func TestCreateRegion_StatusError(t *testing.T) {
	var body string
	server := gqlServer(t, `{"data":{"groupMutation":{"foundationGroupMutation":{"create":[{"foundationGroup":{"foundationGroupId":""},"status":{"status":"FAILED","errorMessage":"name taken"}}]}}}}`, &body)
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	_, err = NewGraphQLClient(client, server.URL, true).CreateRegion(context.Background(), "test")
	require.ErrorContains(t, err, "name taken")
}

func TestQueryRegions(t *testing.T) {
	var body string
	server := gqlServer(t, `{"data":{"entityQuery":{"queryEntities":{"entities":[
		{"entityId":"g1","entityName":"emea","entitiesIn":{"entities":[{"entityId":"f1","entityName":"foundation-a"}]}},
		{"entityId":"g2","entityName":"apac","entitiesIn":{"entities":[]}}]}}}}`, &body)
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	groups, err := NewGraphQLClient(client, server.URL, true).QueryRegions(context.Background())
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "emea", groups[0].EntityName)
	require.Len(t, groups[0].Foundations, 1)
	assert.Equal(t, "foundation-a", groups[0].Foundations[0].EntityName)
	assert.Empty(t, groups[1].Foundations)
	assert.Contains(t, body, "Tanzu.Platform.FoundationGroup")
}

func TestAttachFoundationInGroup(t *testing.T) {
	var bodies []string
	responses := []string{
		`{"data":{"entityQuery":{"queryEntities":{"entities":[{"entityId":"vrn/group-1","entityName":"emea","entitiesIn":{"entities":[]}}]}}}}`,
		`{"data":{"fleetManagementMutation":{"createFoundation":{"foundationEntityId":"vrn/f-1","status":"SUCCESS","errorMessage":null}}}}`,
		`{"data":{"fleetManagementMutation":{"generateFoundationDeploymentProperties":{"deploymentProperties":[
			{"name":"oauth_client_id","value":"cid"},{"name":"oauth_client_secret","value":"sec"},
			{"name":"ingestion_url","value":"https://ingest"},{"name":"hub_ca_certificate","value":"cert"},
			{"name":"foundation_id","value":"fid"},{"name":"collector_id","value":"col"},
			{"name":"csp_org_id","value":"org"},{"name":"frpc_remote_port","value":0},{"name":"ignored","value":"x"}],
			"foundation":{"entityId":"vrn/f-1","entityName":"my-foundation"}}}}}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responses[len(bodies)-1]))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	result, err := NewGraphQLClient(client, server.URL, true).AttachFoundationInRegion(context.Background(), "my-foundation", "emea")
	require.NoError(t, err)
	require.Len(t, bodies, 3)
	assert.Contains(t, bodies[1], "mutation createFoundation(")
	assert.Contains(t, bodies[1], `"regionEntityId":"vrn/group-1"`)
	assert.Contains(t, bodies[1], "$regionEntityId:EntityId")
	assert.Contains(t, bodies[2], `"foundationId":"vrn/f-1"`)
	assert.Contains(t, bodies[2], `"generateOAuthApp":true`)

	assert.Equal(t, "my-foundation", result["foundationName"])
	assert.Equal(t, "cid", result["oauth-app-id"])
	assert.Equal(t, "sec", result["oauth-app-secret"])
	assert.Equal(t, "https://ingest", result["ingestion_url"])
	assert.Equal(t, "cert", result["caBundle"])
	assert.Equal(t, "fid", result["foundation-id"])
	assert.Equal(t, "col", result["collector-id"])
	assert.Equal(t, "org", result["org-id"])
	assert.NotContains(t, result, "ignored")
	assert.NotContains(t, result, "frpc_remote_port")
}

func TestAttachFoundationInGroup_GroupNotFound(t *testing.T) {
	var body string
	server := gqlServer(t, `{"data":{"entityQuery":{"queryEntities":{"entities":[]}}}}`, &body)
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	_, err = NewGraphQLClient(client, server.URL, true).AttachFoundationInRegion(context.Background(), "f", "nope")
	require.ErrorContains(t, err, `no region found with name "nope"`)
}

func TestDeleteRegion(t *testing.T) {
	for name, resp := range map[string]string{
		"list":   `{"data":{"groupMutation":{"foundationGroupMutation":{"delete":[{"status":{"status":"SUCCESS","errorMessage":null}}]}}}}`,
		"single": `{"data":{"groupMutation":{"foundationGroupMutation":{"delete":{"status":{"status":"SUCCESS","errorMessage":null}}}}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var body string
			server := gqlServer(t, resp, &body)
			defer server.Close()

			client, err := GetAccessToken("user", "pass", server.URL, true)
			require.NoError(t, err)

			require.NoError(t, NewGraphQLClient(client, server.URL, true).DeleteRegion(context.Background(), "vrn/g1"))
			assert.Contains(t, body, `"id":["vrn/g1"]`)
			assert.Contains(t, body, "[EntityId!]!")
		})
	}
}

func TestDeleteRegion_StatusError(t *testing.T) {
	var body string
	server := gqlServer(t, `{"data":{"groupMutation":{"foundationGroupMutation":{"delete":[{"status":{"status":"FAILED","errorMessage":"group not empty"}}]}}}}`, &body)
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	err = NewGraphQLClient(client, server.URL, true).DeleteRegion(context.Background(), "vrn/g1")
	require.ErrorContains(t, err, "group not empty")
}

func TestFindRegion(t *testing.T) {
	var body string
	server := gqlServer(t, `{"data":{"entityQuery":{"queryEntities":{"entities":[
		{"entityId":"g1","entityName":"emea","entitiesIn":{"entities":[{"entityId":"f1","entityName":"foundation-a"}]}},
		{"entityId":"g2","entityName":"dup","entitiesIn":{"entities":[]}},
		{"entityId":"g3","entityName":"dup","entitiesIn":{"entities":[]}}]}}}}`, &body)
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)
	gql := NewGraphQLClient(client, server.URL, true)

	region, err := gql.FindRegion(context.Background(), "emea", "")
	require.NoError(t, err)
	assert.Equal(t, "g1", region.EntityID)
	require.Len(t, region.Foundations, 1)

	region, err = gql.FindRegion(context.Background(), "", "g2")
	require.NoError(t, err)
	assert.Equal(t, "dup", region.EntityName)

	_, err = gql.FindRegion(context.Background(), "dup", "")
	require.ErrorContains(t, err, "2 regions")
	_, err = gql.FindRegion(context.Background(), "", "nope")
	require.ErrorContains(t, err, `with ID "nope"`)
}
