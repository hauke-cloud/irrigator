/*
Copyright 2025 Hauke Mettendorf.

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

package valve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds the configuration for the valve-controller HTTP client.
type Config struct {
	BaseURL        string
	ClientCertFile string
	ClientKeyFile  string
	ServerCAFile   string
}

// Client is an mTLS-secured HTTP client for the valve-controller REST API.
type Client struct {
	http    *http.Client
	baseURL string
	log     *slog.Logger
}

// NewClient creates a new valve-controller client with mutual TLS.
func NewClient(cfg Config, log *slog.Logger) (*Client, error) {
	cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client keypair: %w", err)
	}

	caPEM, err := os.ReadFile(cfg.ServerCAFile)
	if err != nil {
		return nil, fmt.Errorf("read server CA: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse server CA cert from %s", cfg.ServerCAFile)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
		},
	}

	return &Client{
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		log:     log,
	}, nil
}

// OpenValve sends a timed open command to the valve-controller.
func (c *Client) OpenValve(ctx context.Context, name, namespace string, duration time.Duration) (*Action, error) {
	url := fmt.Sprintf("%s/api/v1/valves/%s/open?duration=%s", c.baseURL, name, duration.String())

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build open request for valve %s: %w", name, err)
	}
	if namespace != "" {
		req.Header.Set("X-Namespace", namespace)
	}

	c.log.InfoContext(ctx, "opening valve", "valve", name, "namespace", namespace, "duration", duration)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		var action Action
		if err := json.NewDecoder(resp.Body).Decode(&action); err != nil {
			return nil, fmt.Errorf("decode open response: %w", err)
		}
		return &action, nil
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	case http.StatusUnprocessableEntity:
		var e apiError
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Code == "VALVE_DISABLED" {
			return nil, fmt.Errorf("%w: %s", ErrDisabled, name)
		}
		return nil, fmt.Errorf("valve %s rejected: %s", name, e.Error)
	default:
		return nil, fmt.Errorf("unexpected status %d opening valve %s", resp.StatusCode, name)
	}
}

// GetValve retrieves a valve's current state.
func (c *Client) GetValve(ctx context.Context, name, namespace string) (*Valve, error) {
	url := fmt.Sprintf("%s/api/v1/valves/%s", c.baseURL, name)
	if namespace != "" {
		url += "?namespace=" + namespace
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build get request for valve %s: %w", name, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d getting valve %s", resp.StatusCode, name)
	}

	var v Valve
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode valve response: %w", err)
	}
	return &v, nil
}
