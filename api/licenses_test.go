package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mockLicenseResponse = `{
	"data": {
		"licenseQuery": {
			"queryLicenses": [
				{
					"id": "license-1",
					"key": "ABCD-1234",
					"licenseVersion": "1.0",
					"productId": "tanzu-hub",
					"productDescription": "Tanzu Hub",
					"foundationCount": 3,
					"expiration": "2026-12-31T00:00:00Z",
					"extendedAttributes": [
						{"name": "seats", "value": "100"}
					]
				}
			]
		}
	}
}`

func TestQueryLicenses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		assert.Equal(t, "Bearer mock-token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockLicenseResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	licenses, err := gql.QueryLicenses(context.Background())
	require.NoError(t, err)
	require.Len(t, licenses, 1)

	got := licenses[0]
	assert.Equal(t, "license-1", got.ID)
	assert.Equal(t, "ABCD-1234", got.Key)
	assert.Equal(t, "tanzu-hub", got.ProductID)
	assert.Equal(t, 3, got.FoundationCount)
	require.Len(t, got.ExtendedAttributes, 1)
	assert.Equal(t, "seats", got.ExtendedAttributes[0].Name)
}

const mockAddLicenseResponse = `{
	"data": {
		"licenseMutation": {
			"addLicense": {
				"success": true,
				"verificationFailure": null,
				"license": {
					"id": "license-2",
					"key": "WXYZ-5678",
					"licenseVersion": "1.0",
					"productId": "tanzu-hub",
					"productDescription": "Tanzu Hub",
					"foundationCount": 0,
					"expiration": "2027-12-31T00:00:00Z",
					"extendedAttributes": []
				}
			}
		}
	}
}`

const mockAddLicenseFailResponse = `{
	"data": {
		"licenseMutation": {
			"addLicense": {
				"success": false,
				"verificationFailure": "LICENSE_KEY_ERROR",
				"license": null
			}
		}
	}
}`

func TestAddLicense_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockAddLicenseResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.AddLicense(context.Background(), "WXYZ-5678")
	require.NoError(t, err)

	assert.True(t, result.Success)
	assert.Empty(t, result.VerificationFailure)
	require.NotNil(t, result.License)
	assert.Equal(t, "license-2", result.License.ID)
}

const mockApplyLicenseResponse = `{
	"data": {
		"licenseMutation": {
			"applyLicenseToFoundations": {
				"licenseApplicationResults": [
					{
						"foundationId": "foundation-1",
						"status": "WORKFLOW_STARTED",
						"workflowId": "wf-abc",
						"errors": []
					},
					{
						"foundationId": "foundation-2",
						"status": "ALREADY_APPLIED",
						"workflowId": "",
						"errors": []
					}
				]
			}
		}
	}
}`

func TestApplyLicenseToFoundations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockApplyLicenseResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.ApplyLicenseToFoundations(context.Background(), "license-1", []string{"foundation-1", "foundation-2"})
	require.NoError(t, err)
	require.Len(t, result.LicenseApplicationResults, 2)

	r0 := result.LicenseApplicationResults[0]
	assert.Equal(t, "foundation-1", r0.FoundationID)
	assert.Equal(t, "WORKFLOW_STARTED", r0.Status)
	assert.Equal(t, "wf-abc", r0.WorkflowID)

	r1 := result.LicenseApplicationResults[1]
	assert.Equal(t, "ALREADY_APPLIED", r1.Status)
}

func TestDeleteLicense_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"licenseMutation":{"deleteLicense":{"success":true}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	ok, err := gql.DeleteLicense(context.Background(), "license-1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestDeleteLicense_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"licenseMutation":{"deleteLicense":{"success":false}}}}`))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	ok, err := gql.DeleteLicense(context.Background(), "bad-id")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAddLicense_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockAddLicenseFailResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")

	gql := NewGraphQLClient(client, server.URL, true)
	result, err := gql.AddLicense(context.Background(), "BAD-KEY")
	require.NoError(t, err)

	assert.False(t, result.Success)
	assert.Equal(t, "LICENSE_KEY_ERROR", result.VerificationFailure)
}

const mockFoundationEntitiesResponse = `{
	"data": {
		"entityQuery": {
			"queryEntities": {
				"entities": [
					{"entityId": "vrn/provider:TAS/instance:p-bosh-abc/Foundation:p-bosh-abc", "entityName": "prod-tas", "managementEndpoint": {"managementEndpointId": "aaaa-bbbb", "endpointName": "prod-collector"}},
					{"entityId": "vrn/provider:TAS/instance:1111-2222/Foundation:1111-2222", "entityName": "dev-tas", "managementEndpoint": {"managementEndpointId": "1111-2222", "endpointName": "dev-collector"}}
				]
			}
		}
	}
}`

func TestResolveFoundationEntityIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == TokenEndpoint {
			mockAuthEndpoint(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(mockFoundationEntitiesResponse))
	}))
	defer server.Close()

	client, err := GetAccessToken("user", "pass", server.URL, true)
	require.NoError(t, err, "Failed to get access token")
	gql := NewGraphQLClient(client, server.URL, true)

	ids, err := gql.ResolveFoundationEntityIDs(context.Background(),
		[]string{"prod-tas", "1111-2222", "aaaa-bbbb", "vrn/provider:TAS/instance:x/Foundation:x"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"vrn/provider:TAS/instance:p-bosh-abc/Foundation:p-bosh-abc",
		"vrn/provider:TAS/instance:1111-2222/Foundation:1111-2222",
		"vrn/provider:TAS/instance:p-bosh-abc/Foundation:p-bosh-abc",
		"vrn/provider:TAS/instance:x/Foundation:x",
	}, ids)

	_, err = gql.ResolveFoundationEntityIDs(context.Background(), []string{"missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod-tas")
}
