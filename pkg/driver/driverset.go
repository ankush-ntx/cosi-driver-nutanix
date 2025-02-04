/*
Copyright 2024 Nutanix Inc.

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
	"errors"
	"fmt"
	"sync"

	"k8s.io/klog/v2"
)

// Driverset is a structure holding list of Drivers, that can be added or extracted based on the ID.
type Driverset struct {
	drivers sync.Map
}

// Add is used to add new driver to the Driverset.
func (driverset *Driverset) Add(newDriver Driver) error {
	id := newDriver.DriverId

	if _, ok := driverset.drivers.Load(id); ok {
		errDuplicate := errors.New("driver already exists for id: " + id)
		klog.ErrorS(errDuplicate, "failed to load new configuration for specified objectstore")
		return errDuplicate
	}

	driverset.drivers.Store(id, newDriver)

	return nil
}

// Get is used to get driver from the Driverset.
func (driverset *Driverset) Get(id string) (*Driver, error) {
	d, ok := driverset.drivers.Load(id)
	if !ok {
		errNotFound := errors.New("")
		klog.ErrorS(errNotFound, "failed to retrieve configuration for specified objectstore")
		return nil, errNotFound
	}

	switch d := d.(type) {
	case Driver:
		klog.InfoS("Driver exists", "id", id)

		return &d, nil
	default:
		return nil, fmt.Errorf("failed to retrieve configuration for specified object storage platform: %w", errors.New("invalid type"))
	}
}