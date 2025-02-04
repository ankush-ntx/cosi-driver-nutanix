/*
Copyright 2022 Nutanix Inc.
Licensed under the Apache License, Version 2.0 (the "License");
You may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"errors"
	"strings"

	ntnxIam "nutanix-cosi-driver/pkg/admin"
	s3cli "nutanix-cosi-driver/pkg/util/s3client"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	cosi "sigs.k8s.io/container-object-storage-interface/proto"
)

const (
	correctFormatLength = 2
	separator = ":"
) 

// contains two clients
// 1.) for ntnxIamClientOps : mainly for user related operations
// 2.) for S3 operations : mainly for bucket related operations
type ProvisionerServer struct {
	Name string
	Driverset   *Driverset
	cosi.UnimplementedProvisionerServer
}

// Return new provisioner server.
func NewProvisionerServer(provisioner string, driverset *Driverset) *ProvisionerServer {
	return &ProvisionerServer{
		Name: provisioner,
		Driverset:   driverset,
	}
}

// DriverCreateBucket is a method for creating buckets.
// It is expected to create the same bucket given a bucketName and protocol.
// If the bucket already exists, then it MUST return codes.AlreadyExists.
// Return values
//
//	nil -                   Bucket successfully created
//	codes.AlreadyExists -   Bucket already exists. No more retries
//	non-nil err -           Internal error                                [requeue'd with exponential backoff]
func (s *ProvisionerServer) DriverCreateBucket(ctx context.Context,
	req *cosi.DriverCreateBucketRequest) (*cosi.DriverCreateBucketResponse, error) {
	klog.InfoS("Using Nutanix Object store to create Backend Bucket")

	id := req.Parameters["id"]
	d, _ := s.Driverset.Get(id)

	// Get the name of the bucket from the request which is formed
	// by getting the name from the bucket object which is created
	// by the cosi-central-controller.
	bucketName := req.GetName()
	klog.V(3).InfoS("Creating Bucket", "name", bucketName)

	err := d.S3Client.CreateBucket(bucketName)
	if err != nil {
		// Check to see if the bucket already exists by above API
		klog.ErrorS(err, "failed to create bucket", "bucketName", bucketName)
		return nil, status.Error(codes.Internal, "failed to create bucket")
	}
	klog.InfoS("Successfully created Backend Bucket on Nutanix Objects", "bucketName", bucketName)

	return &cosi.DriverCreateBucketResponse{
		BucketId: strings.Join([]string{id, bucketName}, separator), 
	}, nil
}

func (s *ProvisionerServer) DriverDeleteBucket(ctx context.Context,
	req *cosi.DriverDeleteBucketRequest) (*cosi.DriverDeleteBucketResponse, error) {
	klog.InfoS("Deleting bucket", "id", req.GetBucketId())

	id := getID(req.BucketId)
	bucketName, err := getBucketName(req.BucketId)
	if err != nil {
		klog.ErrorS(err, "invalid bucketID", "bucketID", req.BucketId)
	}
	d, err := s.Driverset.Get(id)
	if err != nil {
		klog.ErrorS(err, "invalid driver id", "id", id)
		return nil, status.Error(codes.InvalidArgument, "invalid driver id")
	}

	_, err = d.S3Client.DeleteBucket(bucketName); 
	if err != nil {
		klog.ErrorS(err, "failed to delete bucket", "bucketID", req.GetBucketId())
		return nil, status.Error(codes.Internal, "failed to delete bucket")
	}
	klog.InfoS("Successfully deleted Bucket", "id", req.GetBucketId())

	return &cosi.DriverDeleteBucketResponse{}, nil
}

func (s *ProvisionerServer) DriverGrantBucketAccess(ctx context.Context,
	req *cosi.DriverGrantBucketAccessRequest) (*cosi.DriverGrantBucketAccessResponse, error) {

	id := req.Parameters["id"]
	d, _ := s.Driverset.Get(id)
	// Form the username for this new user, stored in req which is of the form- "ba-<BucketAccessUUID>"
	userName := req.GetName()
	displayName := d.NtnxIamClient.AccountName + "_" + req.GetName()
	bucketName, err := getBucketName(req.BucketId)
	if err != nil {
		klog.ErrorS(err, "invalid bucketID", "bucketID", req.BucketId)
	}
	klog.InfoS("Granting user accessPolicy to bucket", "userName", userName, "displayName",
		displayName, "bucketName", bucketName)

	// Format : {type: "external", email: <userName>@nutanix.com, displayname: <accountName>_<userName> (optional)}
	user, err := d.NtnxIamClient.CreateUser(ctx, userName, displayName)
	if err != nil {
		klog.ErrorS(err, "failed to create an IAM user for Nutanix Objects")
		return nil, err
	}

	// Fetch Bucket Policy
	policy, err := d.S3Client.GetBucketPolicy(bucketName)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() != "NoSuchBucketPolicy" {
			return nil, status.Error(codes.Internal, "fetching policy failed")
		}
	}

	// Share bucket with the newly created IAM user
	statement := s3cli.NewPolicyStatement().
		WithSID(userName).
		ForPrincipals(userName).
		ForResources(bucketName).
		ForSubResources(bucketName).
		Allows().
		Actions(s3cli.AllowedActions...)
	if policy == nil {
		policy = s3cli.NewBucketPolicy(*statement)
	} else {
		policy = policy.ModifyBucketPolicy(*statement)
	}
	_, err = d.S3Client.PutBucketPolicy(bucketName, *policy)
	if err != nil {
		klog.ErrorS(err, "failed to set policy")
		return nil, status.Error(codes.Internal, "failed to set policy")
	}

	return &cosi.DriverGrantBucketAccessResponse{
		AccountId:   user.Users[0].UUID,
		Credentials: fetchUserCredentials(user, d.NtnxIamClient),
	}, nil
}

func (s *ProvisionerServer) DriverRevokeBucketAccess(ctx context.Context,
	req *cosi.DriverRevokeBucketAccessRequest) (*cosi.DriverRevokeBucketAccessResponse, error) {

	klog.InfoS("Deleting user", "id", req.GetAccountId())

	id := getID(req.BucketId)
	d, err := s.Driverset.Get(id)
	if err != nil {
		klog.ErrorS(err, "invalid driver id", "id", id)

		return nil, status.Error(codes.InvalidArgument, "invalid driver id")
	}

	err = d.NtnxIamClient.RemoveUser(ctx, req.GetAccountId())
	if err != nil {
		klog.ErrorS(err, "failed to delete user")
	}
	return &cosi.DriverRevokeBucketAccessResponse{}, nil
}

func fetchUserCredentials(user ntnxIam.NutanixUserResp, ntnxIamClient *ntnxIam.API) map[string]*cosi.CredentialDetails {

	secretsMap := make(map[string]string)
	secretsMap["accessKeyID"] = user.Users[0].BucketsAccessKeys[0].AccessKeyID
	secretsMap["accessSecretKey"] = user.Users[0].BucketsAccessKeys[0].SecretAccessKey
	secretsMap["endpoint"] = ntnxIamClient.Endpoint
	// region mapping needs to be updated
	secretsMap["region"] = ntnxIamClient.Region

	creds := &cosi.CredentialDetails{
		Secrets: secretsMap,
	}

	credDetailsMap := make(map[string]*cosi.CredentialDetails)
	credDetailsMap["s3"] = creds
	return credDetailsMap
}

func getID(s string) string {
	id := strings.Split(s, separator)

	if len(id) < correctFormatLength {
		return ""
	}

	return id[0]
}

func getBucketName(s string) (string, error) {
	list := strings.SplitN(s, separator, correctFormatLength)

	if len(list) != correctFormatLength || list[1] == "" {
		return "", errors.New("invalid bucketID")
	}

	return list[1], nil
}
