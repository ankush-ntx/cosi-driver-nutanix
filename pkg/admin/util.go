package admin

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"nutanix-cosi-driver/pkg/util/config"

	"k8s.io/klog/v2"
	// "strings"
)

const (
	defaultNutanixRegion = "us-east-1"
	defaultAccountName   = "ntnx-cosi-iam-user"
)

var (
	errNoEndpoint        = errors.New("Nutanix object store instance endpoint not set")
	errNoPCEndpoint      = errors.New("Prism Central endpoint for IAM user management not set")
	errInvalidPCEndpoint = errors.New("Prism Central endpoint for IAM user management is invalid")
	errNoPCUsername      = errors.New("Prism Central username for IAM user management not set")
	errNoPCPassword      = errors.New("Prism Central password for IAM user management not set")
)

// HTTPClient interface that conforms to that of the http package's Client.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// API struct for New Client
type API struct {
	Endpoint    string
	PCEndpoint  string
	PCUsername  string
	PCPassword  string
	Region      string
	AccountName string
	HTTPClient  HTTPClient
}

// New returns client for Nutanix object store
func New(cfg *config.Connection, httpClient HTTPClient) (*API, error) {
	klog.InfoS("Creating IAM Client for driver", "driverId", cfg.Id)

	// validate endpoint
	if cfg.ObjectStore.Endpoint == "" {
		return nil, errNoEndpoint
	}

	// validate pc endpoint
	if cfg.PrismCentral.Endpoint == "" {
		return nil, errNoPCEndpoint
	}
	err := ValidateEndpoint(cfg.PrismCentral.Endpoint)
	if err != nil {
		klog.ErrorS(err, "failed to validate to pc endpoint")
		return nil, errInvalidPCEndpoint
	}

	// validate pc username
	if cfg.PrismCentral.Username == "" {
		return nil, errNoPCUsername
	}

	// validate pc password
	if cfg.PrismCentral.Password == "" {
		return nil, errNoPCPassword
	}

	// set default account_name when empty
	if cfg.AccountName == "" {
		cfg.AccountName = defaultAccountName
	}

	if cfg.Region == "" {
		cfg.Region = defaultNutanixRegion
	}

	// If no client is passed initialize it
	if httpClient == nil {
		// SSL certificate verification turned off
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient = &http.Client{Transport: tr}
	}

	IAMClient := API{
		Endpoint:    cfg.ObjectStore.Endpoint,
		PCEndpoint:  cfg.PrismCentral.Endpoint,
		PCUsername:  cfg.PrismCentral.Username,
		PCPassword:  cfg.PrismCentral.Password,
		Region:      cfg.Region,
		AccountName: cfg.AccountName,
		HTTPClient:  httpClient,
	}

	klog.InfoS("IAM Client created")

	return &IAMClient, nil
}

func extractIP(rawURL string) (string, error) {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %v", err)
	}

	// Resolve the hostname to an IP address
	host := parsedURL.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("failed to resolve hostname to IP: %v", err)
	}

	// Return the first IP address
	if len(ips) > 0 {
		return ips[0].String(), nil
	}

	return "", fmt.Errorf("no IP addresses found for hostname: %s", host)
}

// Validate endpoint is of form <ip or hostname>:<port>
func ValidateEndpoint(endpoint string) error {
	if len(endpoint) == 0 {
		return fmt.Errorf("endpoint is not specified")
	}

	pcip, err := extractIP(endpoint)
	if err != nil {
		return fmt.Errorf("error while extracting IP from endpoint %s, err: %s", endpoint, err)
	}

	// epList[0] should be an IP v4 address
	if _, err := net.ResolveIPAddr("ip", pcip); err != nil {
		return fmt.Errorf("error while resolving IP %s, err: %s", pcip, err)
	}

	return nil
}
