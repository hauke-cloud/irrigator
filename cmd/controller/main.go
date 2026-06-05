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

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	irrigatorv1alpha1 "github.com/hauke-cloud/irrigator/api/v1alpha1"
	"github.com/hauke-cloud/irrigator/internal/api"
	"github.com/hauke-cloud/irrigator/internal/controller"
	"github.com/hauke-cloud/irrigator/internal/scheduler"
	"github.com/hauke-cloud/irrigator/internal/valve"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(irrigatorv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		apiAddr          string
		probeAddr        string
		metricsAddr      string
		leaderElection   bool
		leaderElectionNS string
		valveURL         string
		tlsCert          string
		tlsKey           string
		clientCA         string
		valveClientCert  string
		valveClientKey   string
		valveServerCA    string
	)

	flag.StringVar(&apiAddr, "api-addr", ":8443", "REST API listen address")
	flag.StringVar(&probeAddr, "probe-addr", ":8081", "health probe listen address")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "Prometheus metrics listen address")
	flag.BoolVar(&leaderElection, "leader-election", true, "enable leader election")
	flag.StringVar(&leaderElectionNS, "leader-election-namespace", "", "namespace for leader election lock (defaults to in-cluster namespace)")
	flag.StringVar(&valveURL, "valve-controller-url", "https://valve-controller.mqtt.svc.cluster.local:8443", "valve-controller base URL")
	flag.StringVar(&tlsCert, "tls-cert", "/tls/tls.crt", "TLS certificate for the REST API server")
	flag.StringVar(&tlsKey, "tls-key", "/tls/tls.key", "TLS key for the REST API server")
	flag.StringVar(&clientCA, "client-ca", "/tls-client-ca/ca.crt", "CA certificate used to verify API client certs")
	flag.StringVar(&valveClientCert, "valve-client-cert", "/valve-client-tls/tls.crt", "client certificate for the valve-controller API")
	flag.StringVar(&valveClientKey, "valve-client-key", "/valve-client-tls/tls.key", "client key for the valve-controller API")
	flag.StringVar(&valveServerCA, "valve-server-ca", "/valve-server-ca/ca.crt", "CA certificate of the valve-controller server")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctrl.SetLogger(newLogrAdapter(log))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          leaderElection,
		LeaderElectionID:        "irrigator.iot.hauke.cloud",
		LeaderElectionNamespace: leaderElectionNS,
	})
	if err != nil {
		log.Error("create manager", "error", err)
		os.Exit(1)
	}

	valveClient, err := valve.NewClient(valve.Config{
		BaseURL:        valveURL,
		ClientCertFile: valveClientCert,
		ClientKeyFile:  valveClientKey,
		ServerCAFile:   valveServerCA,
	}, log.With("component", "valve-client"))
	if err != nil {
		log.Error("create valve client", "error", err)
		os.Exit(1)
	}

	sched := scheduler.New(log.With("component", "scheduler"))
	exec := controller.NewExecutor(mgr.GetClient(), valveClient, sched, log.With("component", "executor"))

	if err := (&controller.IrrigationScheduleReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Recorder:  mgr.GetEventRecorderFor("irrigator"),
		Scheduler: sched,
		Executor:  exec,
		Log:       log.With("component", "controller"),
	}).SetupWithManager(mgr); err != nil {
		log.Error("setup controller", "error", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error("add healthz check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error("add readyz check", "error", err)
		os.Exit(1)
	}

	apiServer, err := api.NewServer(api.Config{
		Addr:         apiAddr,
		TLSCertFile:  tlsCert,
		TLSKeyFile:   tlsKey,
		ClientCAFile: clientCA,
		K8sClient:    mgr.GetClient(),
		Executor:     exec,
		Log:          log.With("component", "api"),
	})
	if err != nil {
		log.Error("create API server", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return apiServer.Start(gctx) })
	g.Go(func() error { return mgr.Start(gctx) })

	if err := g.Wait(); err != nil {
		log.Error("component exited with error", "error", err)
		os.Exit(1)
	}

	sched.Stop()
}
