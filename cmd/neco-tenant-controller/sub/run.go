package sub

import (
	"fmt"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	multitenancyv1beta1 "github.com/cybozu-go/neco-tenant-controller/api/v1beta1"
	"github.com/cybozu-go/neco-tenant-controller/controllers"
	"github.com/cybozu-go/neco-tenant-controller/hooks"
	"github.com/cybozu-go/neco-tenant-controller/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func subMain(ns, addr string, port int) error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&options.zapOpts)))
	logger := ctrl.Log.WithName("setup")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return fmt.Errorf("unable to add client-go objects: %w", err)
	}
	if err := multitenancyv1beta1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("unable to add neco-tenant-controller objects: %w", err)
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
		Scheme:                  scheme,
		NewClient:               NewCachingClient,
		MetricsBindAddress:      options.metricsAddr,
		HealthProbeBindAddress:  options.probeAddr,
		LeaderElection:          true,
		LeaderElectionID:        options.leaderElectionID,
		LeaderElectionNamespace: ns,
		Host:                    addr,
		Port:                    port,
		CertDir:                 options.certDir,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	if err := cfg.Validate(mgr.GetRESTMapper()); err != nil {
		return fmt.Errorf("invalid configurations: %w", err)
	}

	if err := (&controllers.TenantReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create Namespace controller: %w", err)
	}

	if err := (&controllers.ApplicationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create Namespace controller: %w", err)
	}

	dec, err := admission.NewDecoder(scheme)
	if err != nil {
		return fmt.Errorf("unable to create admission decoder: %w", err)
	}
	hooks.SetupTenantWebhook(mgr, dec)
	hooks.SetupApplicationWebhook(mgr, dec)
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		return fmt.Errorf("problem running manager: %s", err)
	}
	return nil
}

// NewCachingClient is an alternative implementation of controller-runtime's
// default client for manager.Manager.
// https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/cluster#DefaultNewClient
//
// The only difference is that this implementation sets `CacheUnstructured` to `true` to
// cache unstructured objects.
func NewCachingClient(cache cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}

	return client.NewDelegatingClient(client.NewDelegatingClientInput{
		CacheReader:       cache,
		Client:            c,
		UncachedObjects:   uncachedObjects,
		CacheUnstructured: true,
	})
}
