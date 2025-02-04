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
	"strings"

	ntnxIam "nutanix-cosi-driver/pkg/admin"
	"nutanix-cosi-driver/pkg/util/config"
	"nutanix-cosi-driver/pkg/util/s3client"

	"k8s.io/klog/v2"
	cosi "sigs.k8s.io/container-object-storage-interface/sidecar/pkg/provisioner"
)

type Driver struct {
	DriverId      string
	S3Client      *s3client.S3Agent
	NtnxIamClient *ntnxIam.API
}

// Creates a new driver for COSI API with provisioner and identity servers.
func New(ctx context.Context, config *config.Config, provisionerName, driverAddress string) (*cosi.COSIProvisionerServer, error) {
	driverset := &Driverset{}

	// Add virtual driver instance corresponding to a connection to the driverset.
	for _, cfg := range config.Connections {
		klog.InfoS("Traversing through config connections", "driverId", cfg.Id)

		// Create new S3 Client.
		s3Client, err := s3client.NewS3Agent(&cfg, true)
		if err != nil {
			klog.ErrorS(err, "failed to create S3 Client", "driverId", cfg.Id)
			return nil, err
		}
		klog.InfoS("S3 Client created for driver", "driverId", cfg.Id)

		// Create new Nutanix IAM Client.
		ntnxIamClient, err := ntnxIam.New(&cfg, nil)
		if err != nil {
			klog.ErrorS(err, "failed to create IAM Client", "driverId", cfg.Id)
			return nil, err
		}
		klog.InfoS("IAM Client created for driver", "driverId", cfg.Id)

		// Validate new driver name.
		if strings.Contains(cfg.Id, separator) {
			klog.ErrorS(errInvalidDriverId, "driver id contains separator", "driverId", cfg.Id, "separator", separator)
			return nil, err
		}

		driver := &Driver{
			DriverId:      cfg.Id,
			S3Client:      s3Client,
			NtnxIamClient: ntnxIamClient,
		}

		klog.InfoS("Successfully created driver", "driver", driver.DriverId)

		err = driverset.Add(*driver)
		if err != nil {
			klog.ErrorS(err, "failed to add driver to driverset", "driverId", cfg.Id)
			return nil, err
		}
	}

	klog.Info("Driverset successfully created")

	// Setup provisioner and identity servers.
	provisionerServer := NewProvisionerServer(provisionerName, driverset)
	identityServer := NewIdentityServer(provisionerName)

	server, err := cosi.NewDefaultCOSIProvisionerServer(driverAddress,
		identityServer,
		provisionerServer)
	if err != nil {
		klog.ErrorS(err, "failed to create COSI Provisioner Server")
		return nil, err
	}

	return server, nil
}
