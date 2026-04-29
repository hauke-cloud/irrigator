/*
Copyright 2026 hauke.cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alerts

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/hauke-cloud/iot-api/alerts"
)

// Client is a client for the alerts API
type Client struct {
	baseURL    string
	httpClient *http.Client
	log        *zap.Logger
}

// Config holds the configuration for the alerts client
type Config struct {
	BaseURL    string
	CertFile   string
	KeyFile    string
	CAFile     string
	SkipVerify bool
}

// NewClient creates a new alerts API client
func NewClient(config Config, log *zap.Logger) (*Client, error) {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert: %w", err)
	}

	// Load CA cert if provided
	caCertPool := x509.NewCertPool()
	if config.CAFile != "" {
		caCert, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to parse CA cert")
		}
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: config.SkipVerify,
	}

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &Client{
		baseURL:    config.BaseURL,
		httpClient: httpClient,
		log:        log,
	}, nil
}

// GetAlerts retrieves all alerts
func (c *Client) GetAlerts(ctx context.Context) (*alerts.AlertsResponse, error) {
	url := fmt.Sprintf("%s/alerts", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var alertsResp alerts.AlertsResponse
	if err := json.NewDecoder(resp.Body).Decode(&alertsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &alertsResp, nil
}

// GetAlertsBySensorType retrieves alerts filtered by sensor type
func (c *Client) GetAlertsBySensorType(ctx context.Context, sensorType string) ([]alerts.AlertDevice, error) {
	url := fmt.Sprintf("%s/alerts?sensor_type=%s", c.baseURL, sensorType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var alertsResp alerts.AlertsResponse
	if err := json.NewDecoder(resp.Body).Decode(&alertsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return alertsResp.Devices, nil
}
