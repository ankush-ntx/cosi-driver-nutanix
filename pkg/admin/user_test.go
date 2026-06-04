package admin_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nutanix-core/k8s-ntnx-object-cosi/pkg/admin"
	mocks "github.com/nutanix-core/k8s-ntnx-object-cosi/tests/fakes"

	"github.com/stretchr/testify/assert"
)

var (
	ctx     = context.Background()
	baseApi = admin.API{
		PCEndpoint: "https://pc.example.com",
		PCUsername: "admin",
		PCPassword: "password",
	}
)

func TestCreateUser(t *testing.T) {
	mockUsername := "testuser"
	mockDisplayName := "Test User"
	mockRespBody := `{
		"users": [{
			"username": "testuser",
			"display_name": "Test User",
			"type": "external",
			"created_time": "2025-01-01T00:00:00Z",
			"last_updated_time": "2025-01-01T00:00:00Z",
			"tenant_id": "tenant-id",
			"uuid": "user-uuid",
			"buckets_access_keys": [{
				"access_key_id": "access-key",
				"secret_access_key": "secret-key",
				"created_time": "2025-01-01T00:00:00Z"
			}]
		}]
	}`

	t.Run("TestCreateUser_Success", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(mockRespBody)),
				}, nil
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		resp, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.NoError(t, err)
		assert.Equal(t, mockUsername, resp.Users[0].Username)
		assert.Equal(t, "access-key", resp.Users[0].BucketsAccessKeys[0].AccessKeyID)
		assert.NotNil(t, len(resp.Users[0].BucketsAccessKeys))
	})

	t.Run("TestCreateUser_UserNotCreated", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				body := `{
					"users": [{
						"buckets_access_keys": []
					}]
				}`
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(body)),
				}, nil
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		resp, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Equal(t, admin.NutanixUserResp{}, resp)
		assert.Contains(t, err.Error(), "user not created")
	})

	t.Run("TestCreateUser_MissingUsername", func(t *testing.T) {
		api := &admin.API{}
		_, err := api.CreateUser(ctx, "", mockDisplayName)
		assert.Contains(t, err.Error(), "username not set")
	})

	t.Run("TestCreateUser_CreateRequestError", func(t *testing.T) {
		api := baseApi
		api.PCEndpoint = "://"

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create http request")
	})

	t.Run("TestCreateUser_SendRequestError", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("failed to send http request")
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send http request")
	})

	t.Run("TestCreateUser_Non200Response", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				errMsg := `"error":"internal server error"`
				return &http.Response{
					StatusCode: 500,
					Status:     "500 Internal Server Error",
					Body:       io.NopCloser(bytes.NewBufferString(errMsg)),
				}, nil
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "non-200 response")
	})

	t.Run("TestCreateUser_UnmarshalError", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString("{bad json")),
				}, nil
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal")
	})

	// The IAM proxy returns this exact phrase in the error payload when an
	// external user with the requested username already exists. CreateUser
	// is expected to recover by fetching the existing user and minting a
	// new access key for them, returning a successful NutanixUserResp.
	createUserExistsBody := `{
		"users": [{
			"buckets_access_keys": null,
			"code": 1009,
			"message": "User and associated access key already exist for username: testuser",
			"type": "external",
			"username": "testuser"
		}]
	}`

	listUsersBody := `{
		"users": [
			{
				"username": "someoneelse",
				"uuid": "other-uuid",
				"type": "external"
			},
			{
				"username": "testuser",
				"uuid": "existing-user-uuid",
				"type": "external"
			}
		]
	}`

	newAccessKeyBody := `{
		"access_key_id": "new-access-key",
		"access_key_name": "buckets-access-key-fallback",
		"created_time": "2026-05-06T08:48:19Z",
		"secret_access_key": "new-secret-key"
	}`

	t.Run("TestCreateUser_AlreadyExists_FallbackSuccess", func(t *testing.T) {
		var sawListUsers, sawCreateAccessKey bool
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/buckets_access_keys"):
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(createUserExistsBody)),
					}, nil
				case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/users"):
					sawListUsers = true
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(listUsersBody)),
					}, nil
				case req.Method == "POST" && req.URL.Path == "/oss/iam_proxy/users/existing-user-uuid/buckets_access_keys":
					sawCreateAccessKey = true
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(newAccessKeyBody)),
					}, nil
				}
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		resp, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.NoError(t, err)
		assert.True(t, sawListUsers, "list users endpoint should have been called")
		assert.True(t, sawCreateAccessKey, "create access key endpoint should have been called")
		assert.Equal(t, "testuser", resp.Users[0].Username)
		assert.Equal(t, "existing-user-uuid", resp.Users[0].UUID)
		assert.Equal(t, "new-access-key", resp.Users[0].BucketsAccessKeys[0].AccessKeyID)
		assert.Equal(t, "new-secret-key", resp.Users[0].BucketsAccessKeys[0].SecretAccessKey)
	})

	t.Run("TestCreateUser_AlreadyExists_UserNotInList", func(t *testing.T) {
		emptyListBody := `{ "users": [ { "username": "someoneelse", "uuid": "other-uuid" } ] }`

		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/buckets_access_keys"):
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(createUserExistsBody)),
					}, nil
				case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/users"):
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(emptyListBody)),
					}, nil
				}
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in user list")
	})

	t.Run("TestCreateUser_AlreadyExists_ListUsersFails", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/buckets_access_keys"):
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(createUserExistsBody)),
					}, nil
				case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/users"):
					return &http.Response{
						StatusCode: 500,
						Body:       io.NopCloser(bytes.NewBufferString(`{"error":"boom"}`)),
					}, nil
				}
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to list existing users")
	})

	t.Run("TestCreateUser_SendsAPIKeyHeader", func(t *testing.T) {
		// Service Account auth path: when the API has PCAPIKey set,
		// every request to the IAM proxy must carry the
		// X-ntnx-api-key header and must NOT fall back to Basic Auth.
		var capturedHeader, capturedAuth string
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				capturedHeader = req.Header.Get("X-ntnx-api-key")
				capturedAuth = req.Header.Get("Authorization")
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(mockRespBody)),
				}, nil
			},
		}

		api := admin.API{
			PCEndpoint: "https://pc.example.com",
			PCAPIKey:   "test-key",
			HTTPClient: mockClient,
		}

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.NoError(t, err)
		assert.Equal(t, "test-key", capturedHeader)
		assert.Empty(t, capturedAuth)
	})

	t.Run("TestCreateUser_SendsBasicAuthWhenNoAPIKey", func(t *testing.T) {
		// Regression: with username/password only the legacy Basic
		// Auth header must still be set, and the API key header must
		// be absent.
		var capturedHeader, capturedAuth string
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				capturedHeader = req.Header.Get("X-ntnx-api-key")
				capturedAuth = req.Header.Get("Authorization")
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(mockRespBody)),
				}, nil
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.NoError(t, err)
		assert.Empty(t, capturedHeader)
		assert.True(t, strings.HasPrefix(capturedAuth, "Basic "), "expected Basic Auth header, got %q", capturedAuth)
	})

	t.Run("TestCreateUser_AlreadyExists_AccessKeyCreateFails", func(t *testing.T) {
		mockClient := mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/buckets_access_keys"):
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(createUserExistsBody)),
					}, nil
				case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/oss/iam_proxy/users"):
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(listUsersBody)),
					}, nil
				case req.Method == "POST" && req.URL.Path == "/oss/iam_proxy/users/existing-user-uuid/buckets_access_keys":
					return &http.Response{
						StatusCode: 500,
						Body:       io.NopCloser(bytes.NewBufferString(`{"error":"server error"}`)),
					}, nil
				}
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			},
		}

		api := baseApi
		api.HTTPClient = mockClient

		_, err := api.CreateUser(ctx, mockUsername, mockDisplayName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create access key for existing user")
	})

}

func TestRemoveUser(t *testing.T) {
	ctx := context.Background()

	t.Run("TestRemoveUser_MissingUUID", func(t *testing.T) {
		api := baseApi
		api.HTTPClient = mocks.MockHTTPClient{}

		err := api.RemoveUser(ctx, "")
		assert.Contains(t, err.Error(), "user UUID not set")
	})

	t.Run("TestRemoveUser_CreateRequestError", func(t *testing.T) {
		api := baseApi
		api.PCEndpoint = "://"

		err := api.RemoveUser(ctx, "some-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create http request")
	})

	t.Run("TestRemoveUser_SendRequestError", func(t *testing.T) {
		api := baseApi
		api.HTTPClient = mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("http client error")
			},
		}
		err := api.RemoveUser(ctx, "some-id")
		assert.Contains(t, err.Error(), "failed to send http request")
	})

	t.Run("TestRemoveUser_404Response", func(t *testing.T) {
		api := baseApi
		api.HTTPClient = mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				body := `{
					"message": "Requested user does not exist.",
					"code": 404
				}`
				resp := &http.Response{
					StatusCode: 404,
					Status:     "404 Not Found",
					Body:       io.NopCloser(bytes.NewBufferString(body)),
				}
				return resp, nil
			},
		}
		err := api.RemoveUser(ctx, "some-id")
		assert.NoError(t, err)
	})

	t.Run("TestRemoveUser_Non204Response", func(t *testing.T) {
		api := baseApi
		api.HTTPClient = mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				body := `{
					"message": "Requested user does not exist.",
					"code": 500
				}`
				resp := &http.Response{
					StatusCode: 500,
					Status:     "500 Internal Server Error",
					Body:       io.NopCloser(bytes.NewBufferString(body)),
				}
				return resp, nil
			},
		}
		err := api.RemoveUser(ctx, "some-id")
		assert.Contains(t, err.Error(), "non-204 response")
	})

	t.Run("TestRemoveUser_Success", func(t *testing.T) {
		api := baseApi
		api.HTTPClient = mocks.MockHTTPClient{
			DoFunc: func(req *http.Request) (*http.Response, error) {
				resp := &http.Response{
					StatusCode: 204,
					Body:       io.NopCloser(bytes.NewBufferString("")),
				}
				return resp, nil
			},
		}
		err := api.RemoveUser(ctx, "some-id")
		assert.NoError(t, err)
	})
}
