/*
Copyright 2022.

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
	"crypto/tls"
	"flag"
	"os"
	"strings"

	mutating "github.com/confidential-containers/cloud-api-adaptor/src/webhook/pkg/mutating"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8443", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Build TLS options from operator-injected env vars.
	// The operator is the single source of truth for the cluster TLS profile.
	var webhookTLSOpts []func(*tls.Config)
	var cipherSuiteNames []string
	if cs := os.Getenv("TLS_CIPHER_SUITES"); cs != "" {
		cipherSuiteNames = strings.Split(cs, ",")
	}

	profile, err := parseTLSOptions(os.Getenv("TLS_MIN_VERSION"), cipherSuiteNames)
	if err != nil {
		setupLog.Error(err, "invalid TLS configuration")
		os.Exit(1)
	}
	if profile != nil {
		webhookTLSOpts = append(webhookTLSOpts, func(c *tls.Config) {
			c.MinVersion = profile.MinVersion
			c.CipherSuites = profile.CipherSuites
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:    metricsAddr,
			SecureServing:  true,
			FilterProvider: filters.WithAuthenticationAndAuthorization,
			TLSOpts:        webhookTLSOpts,
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port:    9443,
				TLSOpts: webhookTLSOpts,
			},
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "a3663802.confidential-containers",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	setupLog.Info("Setting up webhook server")
	podMutator := &mutating.PodMutator{
		Client:  mgr.GetClient(),
		Decoder: admission.NewDecoder(mgr.GetScheme()),
	}

	mgr.GetWebhookServer().Register("/mutate-v1-pod", &webhook.Admission{Handler: podMutator})

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
