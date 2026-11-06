package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockManagementEndpointsResponse = `{
	"data": {
		"managementEndpointQuery": {
			"queryManagementEndpointCollectors": {
				"managementEndpointCollectors": [
					{
						"id": "collector-1",
						"name": "my-collector",
						"type": "FOUNDATION",
						"managementEndpointCollectorTypeVersion": "1.2.3",
						"latestAvailableVersion": "1.3.0",
						"healthStatus": "HEALTHY",
						"status": "ACTIVE",
						"lastUpdateTime": "2025-03-01T00:00:00Z",
						"managementEndpoint": {
							"managementEndpointId": "foundation-abc",
							"endpointName": "my-foundation",
							"environment": "production"
						}
					},
					{
						"id": "collector-2",
						"name": "staging-collector",
						"type": "FOUNDATION",
						"managementEndpointCollectorTypeVersion": null,
						"latestAvailableVersion": "1.3.0",
						"healthStatus": "HEALTHY",
						"status": "ACTIVE",
						"lastUpdateTime": "2025-07-01T00:00:00Z",
						"managementEndpoint": {
							"managementEndpointId": "foundation-xyz",
							"endpointName": "staging-foundation",
							"environment": "staging"
						}
					}
				]
			}
		}
	}
}`

func TestQueryManagementEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockManagementEndpointsResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	collectors, err := gql.QueryManagementEndpoints(context.Background())
	require.NoError(t, err)
	require.Len(t, collectors, 2)

	c0 := collectors[0]
	assert.Equal(t, "collector-1", c0.ID)
	assert.Equal(t, "1.2.3", c0.ManagementEndpointCollectorTypeVersion)
	assert.Equal(t, "2025-03-01T00:00:00Z", c0.LastUpdateTime)
	assert.Equal(t, "my-foundation", c0.ManagementEndpoint.EndpointName)

	c1 := collectors[1]
	assert.Empty(t, c1.ManagementEndpointCollectorTypeVersion)
}

const mockAttachCollectorResponse = `{
	"data": {
		"managementEndpointMutation": {
			"attachSelfManagedManagementEndpointCollector": {
				"id": "collector-new",
				"name": "my-new-collector",
				"deploymentInstall": "install-script",
				"deploymentProperties": [],
				"managementEndpoint": {
					"managementEndpointId": "foundation-abc",
					"endpointName": "my-foundation",
					"environment": "production"
				},
				"managementEndpointCollectorCredentials": {
					"properties": {
						"clientId": "client-abc",
						"clientSecret": "secret-xyz"
					}
				}
			}
		}
	}
}`

func TestDeleteFoundationByName_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		callCount++
		if callCount == 1 {
			// First call: list foundations; the collector name differs from the foundation name
			_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[{"id":"me-abc","name":"generated-name","managementEndpoint":{"endpointName":"my-foundation"}}]}}}}`))
		} else {
			// Second call: detach by id
			_, _ = w.Write([]byte(`{"data":{"managementEndpointMutation":{"detachManagementEndpointCollector":{"id":"me-abc"}}}}`))
		}
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	err = gql.DeleteFoundationByName(context.Background(), "my-foundation")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "Expected 2 GraphQL calls")
}

func TestDeleteFoundationByName_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	err = gql.DeleteFoundationByName(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestAttachSelfManagedCollector(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		callCount++
		if callCount == 1 {
			// Pre-flight existence check — no existing collectors
			_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[]}}}}`))
		} else {
			// Attach mutation
			_, _ = w.Write([]byte(mockAttachCollectorResponse))
		}
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.AttachSelfManagedCollector(context.Background(), "my-new-collector", "Kubernetes", "foundation-abc")
	require.NoError(t, err)

	assert.Equal(t, "collector-new", result["id"])
	assert.Equal(t, "my-new-collector", result["name"])
	assert.Equal(t, "my-foundation", result["foundationName"])
}

func TestUpdateSelfManagedCollector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockAttachCollectorResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.UpdateSelfManagedCollector(context.Background(), "my-new-collector", "Kubernetes", "foundation-abc")
	require.NoError(t, err)

	assert.Equal(t, "collector-new", result["id"])
	assert.Equal(t, "my-new-collector", result["name"])
	assert.Equal(t, "my-foundation", result["foundationName"])
}

func TestAttachSelfManagedCollector_AlreadyExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Pre-flight existence check — collector already exists
		_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[{"id":"existing-id"}]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	_, err = gql.AttachSelfManagedCollector(context.Background(), "my-new-collector", "Kubernetes", "foundation-abc")
	assert.ErrorContains(t, err, "already exists")
}

func TestDeleteFoundationByName_Ambiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"managementEndpointQuery":{"queryManagementEndpointCollectors":{"managementEndpointCollectors":[
			{"id":"a","name":"x","managementEndpoint":{"endpointName":"dup"}},
			{"id":"b","name":"y","managementEndpoint":{"endpointName":"dup"}}]}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err)

	err = NewGraphQLClient(client, server.URL, true).DeleteFoundationByName(context.Background(), "dup")
	require.ErrorContains(t, err, "2 foundations found")
}
