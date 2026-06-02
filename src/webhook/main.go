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
	"fmt"
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

// tlsVersionFromString maps a TLS version string to the corresponding uint16.
// Returns tls.VersionTLS12 for empty input. Rejects TLS 1.0 and 1.1.
func tlsVersionFromString(v string) (uint16, error) {
	switch v {
	case "", "VersionTLS12":
		return tls.VersionTLS12, nil
	case "VersionTLS13":
		return tls.VersionTLS13, nil
	case "VersionTLS10", "VersionTLS11":
		return 0, fmt.Errorf("invalid minVersion %q: TLS 1.0 and 1.1 are not supported, use VersionTLS12 or VersionTLS13", v)
	default:
		return 0, fmt.Errorf("unknown TLS version %q, use VersionTLS12 or VersionTLS13", v)
	}
}

// tlsCipherSuitesFromNames maps IANA cipher suite names to their crypto/tls uint16 IDs.
func tlsCipherSuitesFromNames(names []string) ([]uint16, error) {
	all := append(tls.CipherSuites(), tls.InsecureCipherSuites()...)
	byName := make(map[string]uint16, len(all))
	for _, cs := range all {
		byName[cs.Name] = cs.ID
	}
	ids := make([]uint16, 0, len(names))
	for _, n := range names {
		id, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("unknown cipher suite %q", n)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
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
	minVersionStr := os.Getenv("TLS_MIN_VERSION")
	cipherSuitesStr := os.Getenv("TLS_CIPHER_SUITES")

	if minVersionStr != "" || cipherSuitesStr != "" {
		minVersion, err := tlsVersionFromString(minVersionStr)
		if err != nil {
			setupLog.Error(err, "invalid TLS_MIN_VERSION")
			os.Exit(1)
		}

		var cipherSuiteIDs []uint16
		if cipherSuitesStr != "" {
			if minVersion == tls.VersionTLS13 {
				setupLog.Error(fmt.Errorf("cipher suites may not be specified when TLS_MIN_VERSION is VersionTLS13"), "invalid TLS configuration")
				os.Exit(1)
			}
			names := strings.Split(cipherSuitesStr, ",")
			cipherSuiteIDs, err = tlsCipherSuitesFromNames(names)
			if err != nil {
				setupLog.Error(err, "invalid TLS_CIPHER_SUITES")
				os.Exit(1)
			}
		}

		webhookTLSOpts = append(webhookTLSOpts, func(c *tls.Config) {
			c.MinVersion = minVersion
			c.CipherSuites = cipherSuiteIDs
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
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
