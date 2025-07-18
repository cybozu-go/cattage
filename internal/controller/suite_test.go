package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/internal/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sCfg *rest.Config
var k8sClient client.Client
var scheme *k8sruntime.Scheme
var testEnv *envtest.Environment

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyTimeout(20 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultConsistentlyDuration(5 * time.Second)
	SetDefaultConsistentlyPollingInterval(100 * time.Millisecond)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			filepath.Join("..", "..", "test", "crd"),
		},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.32.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	k8sCfg = cfg

	scheme = k8sruntime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = cattagev1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	ctx := context.Background()
	ns := &corev1.Namespace{}
	ns.Name = "argocd"
	err = k8sClient.Create(ctx, ns)
	Expect(err).NotTo(HaveOccurred())

	ns = &corev1.Namespace{}
	ns.Name = "sub-1"
	ns.Labels = map[string]string{
		constants.OwnerTenant: "a-team",
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).NotTo(HaveOccurred())

	ns = &corev1.Namespace{}
	ns.Name = "sub-2"
	ns.Labels = map[string]string{
		constants.OwnerTenant: "a-team",
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).NotTo(HaveOccurred())

	ns = &corev1.Namespace{}
	ns.Name = "sub-3"
	ns.Labels = map[string]string{
		constants.OwnerTenant: "a-team",
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).NotTo(HaveOccurred())

	ns = &corev1.Namespace{}
	ns.Name = "sub-4"
	ns.Labels = map[string]string{
		constants.OwnerTenant: "x-team",
	}
	err = k8sClient.Create(ctx, ns)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
