package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	marshalError             = "failed to marshal ntnx http request body"
	unmarshalError           = "failed to unmarshal ntnx http response"
	userAlreadyExistsMessage = "User and associated access key already exist"
	createEndpoint           = "/oss/iam_proxy/buckets_access_keys"
	deleteEndpoint           = "/oss/iam_proxy/users/"
	listUsersEndpoint        = "/oss/iam_proxy/users"
	accessKeysEndpoint       = "/oss/iam_proxy/users/%s/buckets_access_keys"
)

var (
	errMissingUsername = errors.New("username not set")
	errMissingUserID   = errors.New("user UUID not set")
)

type NtnxUserReq struct {
	Users []NtnxUserInfo `json:"users"`
}

type NtnxUserInfo struct {
	Type        string `json:"type"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type NutanixBucketAccessKey struct {
	AccessKeyID     string    `json:"access_key_id"`
	CreatedTime     time.Time `json:"created_time"`
	SecretAccessKey string    `json:"secret_access_key"`
}

type NutanixUser struct {
	BucketsAccessKeys []NutanixBucketAccessKey `json:"buckets_access_keys"`
	CreatedTime       time.Time                `json:"created_time"`
	DisplayName       string                   `json:"display_name"`
	LastUpdatedTime   time.Time                `json:"last_updated_time"`
	TenantID          string                   `json:"tenant_id"`
	Type              string                   `json:"type"`
	Username          string                   `json:"username"`
	UUID              string                   `json:"uuid"`
}

type NutanixUserResp struct {
	Users []NutanixUser `json:"users"`
}

type NutanixUserErrorResp struct {
	Users []struct {
		BucketsAccessKeys interface{} `json:"buckets_access_keys"`
		Code              int         `json:"code"`
		Message           string      `json:"message"`
		Type              string      `json:"type"`
		Username          string      `json:"username"`
	} `json:"users"`
}

// NutanixUsersListResp is the response shape of GET /oss/iam_proxy/users.
// Only the fields we actually need are unmarshaled; the rest of the payload
// is ignored.
type NutanixUsersListResp struct {
	Users []NutanixUserListEntry `json:"users"`
}

type NutanixUserListEntry struct {
	Username string `json:"username"`
	UUID     string `json:"uuid"`
	Type     string `json:"type"`
}

// NutanixAccessKeyResp is the response shape of POST
// /oss/iam_proxy/users/<UUID>/buckets_access_keys.
type NutanixAccessKeyResp struct {
	AccessKeyID     string    `json:"access_key_id"`
	AccessKeyName   string    `json:"access_key_name"`
	CreatedTime     time.Time `json:"created_time"`
	SecretAccessKey string    `json:"secret_access_key"`
}

// Nutanix IAM User
func (api *API) CreateUser(ctx context.Context, username, display_name string) (NutanixUserResp, error) {
	result := NutanixUserResp{}
	if username == "" {
		return result, errMissingUsername
	}

	// Create API
	url := api.PCEndpoint + createEndpoint

	// Request Body
	info := &NtnxUserReq{
		Users: []NtnxUserInfo{
			{
				Type:        "external",
				Username:    username,
				DisplayName: display_name,
			},
		},
	}

	// Converts data struct into json
	data, err := json.Marshal(info)
	if err != nil {
		return result, fmt.Errorf("%s. %w", marshalError, err)
	}

	// Send Request
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return result, fmt.Errorf("failed to create http request. %w", err)
	}

	api.Authenticate(request)
	request.Header.Add("Content-Type", "application/json")
	resp, err := api.HTTPClient.Do(request)
	if err != nil {
		return result, fmt.Errorf("failed to send http request. %w", err)
	}
	defer resp.Body.Close()

	decodedResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("%w", err)
	}

	// Check response status
	if resp.StatusCode != 200 {
		return result, fmt.Errorf("non-200 response: %d - %s", resp.StatusCode, string(decodedResponse))
	}

	// Unmarshal response into Go type
	err = json.Unmarshal(decodedResponse, &result)
	if err != nil {
		return result, fmt.Errorf("%s. %s. %w", unmarshalError, string(decodedResponse), err)
	}

	// Unmarshal function doesn't return an error if attributes are different from the defined struct
	// len(result.Users[0].BucketsAccessKeys) equal to 0, implies that new user wasn't created
	if len(result.Users[0].BucketsAccessKeys) == 0 {
		// Using NutanixUserErrorResp struct to capture error code and error message
		errorResp := NutanixUserErrorResp{}
		err = json.Unmarshal(decodedResponse, &errorResp)
		if err != nil {
			return result, fmt.Errorf("%s. %s. %w", unmarshalError, string(decodedResponse), err)
		}

		if strings.Contains(errorResp.Users[0].Message, userAlreadyExistsMessage) {
			result, err = api.createAccessKeyForExistingUser(ctx, username)
			if err != nil {
				return NutanixUserResp{}, fmt.Errorf("failed to create access key for existing user. %w", err)
			}
			return result, nil
		}

		return NutanixUserResp{}, fmt.Errorf("user not created. errorCode : %d, errorMessage : %s", errorResp.Users[0].Code, errorResp.Users[0].Message)
	}

	return result, nil
}

// listUsers returns all IAM users known to the Nutanix IAM proxy.
// The response payload from the proxy contains many fields; we only decode
// the username/uuid/type triples that the driver needs.
func (api *API) listUsers(ctx context.Context) (NutanixUsersListResp, error) {
	result := NutanixUsersListResp{}

	url := api.PCEndpoint + listUsersEndpoint
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create http request. %w", err)
	}

	api.Authenticate(request)
	resp, err := api.HTTPClient.Do(request)
	if err != nil {
		return result, fmt.Errorf("failed to send http request. %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("%w", err)
	}

	if resp.StatusCode != 200 {
		return result, fmt.Errorf("non-200 response: %d - %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("%s. %s. %w", unmarshalError, string(body), err)
	}

	return result, nil
}

// createAccessKeyForExistingUser is the fallback path used by CreateUser when
// the IAM proxy reports that the user (and an access key for them) already
// exists. It looks up the user's extId by username and mints a fresh access key.
func (api *API) createAccessKeyForExistingUser(ctx context.Context, username string) (NutanixUserResp, error) {
	users, err := api.listUsers(ctx)
	if err != nil {
		return NutanixUserResp{}, fmt.Errorf("failed to list existing users. %w", err)
	}

	var uuid string
	for _, u := range users.Users {
		if u.Username == username {
			uuid = u.UUID
			break
		}
	}
	if uuid == "" {
		return NutanixUserResp{}, fmt.Errorf("user %q reported as existing but not found in user list", username)
	}

	accessKey := NutanixAccessKeyResp{}

	url := api.PCEndpoint + fmt.Sprintf(accessKeysEndpoint, uuid)
	request, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return NutanixUserResp{}, fmt.Errorf("failed to create http request. %w", err)
	}

	api.Authenticate(request)
	request.Header.Add("Content-Type", "application/json")
	resp, err := api.HTTPClient.Do(request)
	if err != nil {
		return NutanixUserResp{}, fmt.Errorf("failed to send http request. %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NutanixUserResp{}, fmt.Errorf("%w", err)
	}

	if resp.StatusCode != 200 {
		return NutanixUserResp{}, fmt.Errorf("non-200 response: %d - %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, &accessKey); err != nil {
		return NutanixUserResp{}, fmt.Errorf("%s. %s. %w", unmarshalError, string(body), err)
	}

	return NutanixUserResp{
		Users: []NutanixUser{
			{
				Username: username,
				UUID:     uuid,
				Type:     "external",
				BucketsAccessKeys: []NutanixBucketAccessKey{
					{
						AccessKeyID:     accessKey.AccessKeyID,
						SecretAccessKey: accessKey.SecretAccessKey,
						CreatedTime:     accessKey.CreatedTime,
					},
				},
			},
		},
	}, nil
}

// RemoveUser removes an user from the object store
func (api *API) RemoveUser(ctx context.Context, uuid string) error {

	if uuid == "" {
		return errMissingUserID
	}

	// Delete API
	delete_url := api.PCEndpoint + deleteEndpoint + string(uuid)
	delete_request, err := http.NewRequest("DELETE", delete_url, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request. %w", err)
	}

	api.Authenticate(delete_request)
	delete_resp, err := api.HTTPClient.Do(delete_request)
	if err != nil {
		return fmt.Errorf("failed to send http request. %w", err)
	}
	defer delete_resp.Body.Close()

	decodedResponse, err := io.ReadAll(delete_resp.Body)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Check response status
	if delete_resp.StatusCode == 404 {
		return nil
	}
	if delete_resp.StatusCode != 204 {
		return fmt.Errorf("non-204 response: %d - %s", delete_resp.StatusCode, string(decodedResponse))
	}
	return nil
}
