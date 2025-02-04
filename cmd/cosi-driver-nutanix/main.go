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

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nutanix-cosi-driver/pkg/driver"
	"nutanix-cosi-driver/pkg/util/config"
	"k8s.io/klog/v2"
)

func init() {
    klog.InitFlags(nil)
	flag.Parse()
}

var (
    configFile = flag.String("config", "/cosi/config.yaml", "path to config file")
)

const (
    cosiSocket = "unix:///var/lib/cosi/cosi.sock"
    driverName = "ntnx.objectstorage.k8s.io"
)

func main() {

	// Create a context that is cancelled when SIGINT or SIGTERM signal is received.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a channel to listen for signals.
	sigs := make(chan os.Signal, 1)
	// Listen for the SIGINT and SIGTERM signals.
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	
	// Create a goroutine to listen for the signals.
	go func() {
		// Wait for a signal.
		sig := <-sigs

		klog.InfoS("Signal received", "type", sig)
		cancel()

		// Exit after 30 seconds
		<-time.After(30 * time.Second)
		os.Exit(1)
	}()

	if err := run(ctx); err != nil {
		klog.ErrorS(err, "Exiting on error")
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	// Load the driver config file.
	cfg, err := config.New(*configFile)
	if err != nil {
		klog.ErrorS(err, "failed to create configuration")
		return err
	}
	klog.InfoS("Config loaded successfully", "configFilePath", *configFile)

	// Create new driver.
	driver, err := driver.New(ctx, cfg, driverName, cosiSocket)
	if err != nil {
		klog.ErrorS(err, "failed to create driver")
		return err
	}
	klog.InfoS("COSI Driver created successfully")

	klog.InfoS("Starting COSI Driver")

	// Start COSI Provisioner driver.
	return driver.Run(ctx)
}
