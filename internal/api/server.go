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

// Package api provides the REST API server for the irrigator controller.
// All routes except /api/v1/healthz and /api/v1/readyz require a client
// certificate signed by the configured client CA (mutual TLS).
package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config holds all configuration for the API server.
type Config struct {
	Addr             string
	TLSCertFile      string
	TLSKeyFile       string
	ClientCAFile     string
	DefaultNamespace string
	K8sClient        client.Client
	Executor         Executor
	Log              *slog.Logger
}

// Server is the irrigator REST API server.
type Server struct {
	cfg    Config
	router http.Handler
	tls    *tls.Config
}

// NewServer constructs and configures the API server. TLS certificates are
// loaded eagerly so misconfiguration is detected at startup.
func NewServer(cfg Config) (*Server, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server keypair: %w", err)
	}

	caPEM, err := os.ReadFile(cfg.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read client CA: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse client CA cert from %s", cfg.ClientCAFile)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.VerifyClientCertIfGiven,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}

	h := &handler{
		k8s:              cfg.K8sClient,
		executor:         cfg.Executor,
		defaultNamespace: cfg.DefaultNamespace,
		log:              cfg.Log,
	}

	return &Server{cfg: cfg, router: newRouter(h), tls: tlsCfg}, nil
}

// Start listens on the configured address until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	ln, err := tls.Listen("tcp", s.cfg.Addr, s.tls)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}

	srv := &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	s.cfg.Log.Info("API server listening", "addr", s.cfg.Addr)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("api server: %w", err)
		}
		return nil
	}
}
