package sub

import (
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/internal/config"
	"github.com/cybozu-go/cattage/internal/controller"
	"github.com/cybozu-go/cattage/internal/hooks"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func subMain(ns, addr string, port int) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options.zapOpts)))
	logger := ctrl.Log.WithName("setup")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return fmt.Errorf("unable to add client-go objects: %w", err)
	}
	if err := cattagev1beta1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("unable to add cattage objects: %w", err)
	}

	cfgData, err := os.ReadFile(options.configFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", options.configFile, err)
	}
	cfg := &config.Config{}
	if err := cfg.Load(cfgData); err != nil {
		return fmt.Errorf("unable to load the configuration file: %w", err)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Client: client.Options{
			Cache: &client.CacheOptions{
				Unstructured: true,
			},
		},
		Metrics: metricsserver.Options{
			BindAddress: options.metricsAddr,
		},
		HealthProbeBindAddress:  options.probeAddr,
		LeaderElection:          true,
		LeaderElectionID:        options.leaderElectionID,
		LeaderElectionNamespace: ns,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    addr,
			Port:    port,
			CertDir: options.certDir,
		}),
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configurations: %w", err)
	}
	ctx := ctrl.SetupSignalHandler()
	if err := controller.SetupIndexForNamespace(ctx, mgr); err != nil {
		return fmt.Errorf("failed to setup indexer for namespaces: %w", err)
	}
	if err := controller.NewTenantReconciler(
		mgr.GetClient(),
		cfg,
	).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create Namespace controller: %w", err)
	}

	hooks.SetupTenantWebhook(mgr, admission.NewDecoder(scheme), cfg)
	hooks.SetupApplicationWebhook(mgr, admission.NewDecoder(scheme), cfg)
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("problem running manager: %s", err)
	}
	return nil
}
