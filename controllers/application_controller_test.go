package controllers

import (
	"context"
	"time"

	"github.com/cybozu-go/neco-tenant-controller/pkg/client"
	tenantconfig "github.com/cybozu-go/neco-tenant-controller/pkg/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Application controller", func() {
	ctx := context.Background()
	var stopFunc func()

	BeforeEach(func() {
		mgr, err := ctrl.NewManager(k8sCfg, ctrl.Options{
			Scheme:             scheme,
			LeaderElection:     false,
			MetricsBindAddress: "0",
			NewClient:          client.NewCachingClient,
		})
		Expect(err).ToNot(HaveOccurred())

		config := &tenantconfig.Config{}
		ar := NewApplicationReconciler(mgr.GetClient(), config)
		err = ar.SetupWithManager(ctx, mgr)
		Expect(err).ToNot(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			err := mgr.Start(ctx)
			if err != nil {
				panic(err)
			}
		}()
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		stopFunc()
		time.Sleep(100 * time.Millisecond)
	})

	It("should sync an application", func() {

	})
})
