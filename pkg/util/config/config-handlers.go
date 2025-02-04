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

package config

import (
	"errors"
	"os"
	"path"
	
	"k8s.io/klog/v2"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Connections []Connection `json:"connections,omitempty" yaml:"connections,omitempty" mapstructure:"connections,omitempty"`
}

type Connection struct {
	Id string `json:"id" yaml:"id" mapstructure:"id"`
	ObjectStore ObjectStore `json:"objectStore,omitempty" yaml:"objectStore,omitempty" mapstructure:"objectStore,omitempty"`
	PrismCentral PrismCentral `json:"prismCentral,omitempty" yaml:"prismCentral,omitempty" mapstructure:"prismCentral,omitempty"`
	AccountName string `json:"accountName,omitempty" yaml:"accountName,omitempty" mapstructure:"accountName,omitempty"`
	Region string `json:"region,omitempty" yaml:"region,omitempty" mapstructure:"region,omitempty"`
}

type ObjectStore struct {
	AccessKey string `json:"accessKey" yaml:"accessKey" mapstructure:"accessKey"`
	SecretKey string `json:"secretKey" yaml:"secretKey" mapstructure:"secretKey"`
	Endpoint string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
}

type PrismCentral struct {
	Endpoint string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	Username string `json:"username,omitempty" yaml:"username,omitempty" mapstructure:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty" mapstructure:"password,omitempty"`
}

// Returns config structure from the provided YAML or JSON file.
func New(filename string) (*Config, error) {
	ext := path.Ext(filename)

	if ext == ".yaml" || ext == ".yml" {
		configByte, err := os.ReadFile(filename)
		if err != nil {
			klog.ErrorS(err, "unable to read YAML config file")
			return nil, err
		}
		klog.InfoS("YAML config file read successfully")

		return NewConfigFromYAML(configByte)

	} else {
		err := errors.New("invalid file extension, should be .json, .yaml or .yml")
		return nil, err
	}
}

// Takes an array of bytes (in YAML format) and unmarshals it to return config structure.
func NewConfigFromYAML(bytes []byte) (*Config, error) {
	cfg := &Config{}

	err := yaml.Unmarshal(bytes, cfg)
	if err != nil {
		klog.Error(err, "unmarshalling of YAML config file failed")
		return nil, err
	}

	klog.InfoS("YAML config file unmarshalled", "config", cfg)

	return cfg, nil
}
