package controller

import (
	"context"
	_ "embed"
	"errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"strings"
	"time"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/internal/accurate"
	"github.com/cybozu-go/cattage/internal/argocd"
	tenantconfig "github.com/cybozu-go/cattage/internal/config"
	"github.com/cybozu-go/cattage/internal/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	k8smetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

//go:embed testdata/appprojecttemplate.yaml
var appProjectTemplate string

//go:embed testdata/rolebindingtemplate.yaml
var roleBindingTemplate string

var _ = Describe("Tenant controller", Ordered, func() {
	ctx := context.Background()
	var stopFunc func()
	var tenantCfg *tenantconfig.Config

	BeforeEach(func() {
		mgr, err := ctrl.NewManager(k8sCfg, ctrl.Options{
			Scheme:         scheme,
			LeaderElection: false,
			Metrics: metricsserver.Options{
				BindAddress: "0",
			},
			Controller: config.Controller{
				SkipNameValidation: ptr.To(true),
			},
			Client: client.Options{
				Cache: &client.CacheOptions{
					Unstructured: true,
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())

		tenantCfg = &tenantconfig.Config{
			Namespace: tenantconfig.NamespaceConfig{
				CommonLabels: map[string]string{
					accurate.LabelTemplate: "init-template",
				},
				CommonAnnotations: map[string]string{
					"hoge": "fuga",
				},
				RoleBindingTemplate: roleBindingTemplate,
			},
			ArgoCD: tenantconfig.ArgoCDConfig{
				Namespace:                           "argocd",
				AppProjectTemplate:                  appProjectTemplate,
				PreventAppCreationInArgoCDNamespace: true,
			},
		}
		tr := NewTenantReconciler(mgr.GetClient(), tenantCfg)
		err = tr.SetupWithManager(mgr)
		Expect(err).ToNot(HaveOccurred())
		err = SetupIndexForNamespace(ctx, mgr)
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

	It("should create root namespaces, rolebindings and an appproject", func() {
		cTeam := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "c-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-c",
					},
				},
				ArgoCD: cattagev1beta1.ArgoCDSpec{
					Repositories: []string{
						"https://github.com/cybozu-go/*",
					},
				},
				ExtraParams: &cattagev1beta1.Params{Data: map[string]interface{}{
					"GitHubTeam": "c-team-gh",
				}},
			},
		}
		err := k8sClient.Create(ctx, cTeam)
		Expect(err).ToNot(HaveOccurred())
		xTeam := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "x-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-x",
						Labels: map[string]string{
							"foo": "bar",
						},
						Annotations: map[string]string{
							"abc": "def",
						},
					},
				},
				ArgoCD: cattagev1beta1.ArgoCDSpec{
					Repositories: []string{
						"https://github.com/cybozu-go/*",
					},
				},
				Delegates: []cattagev1beta1.DelegateSpec{
					{
						Name: "c-team",
						Roles: []string{
							"admin",
						},
					},
				},
				ExtraParams: &cattagev1beta1.Params{Data: map[string]interface{}{
					"GitHubTeam": "x-team-gh",
					"Destinations": []string{
						"extra-namespace-x",
						"extra-namespace-y",
					},
				}},
			},
		}
		err = k8sClient.Create(ctx, xTeam)
		Expect(err).ToNot(HaveOccurred())

		ns := &corev1.Namespace{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "app-x"}, ns); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		Expect(ns.Labels).Should(MatchAllKeys(Keys{
			"kubernetes.io/metadata.name": Equal("app-x"),
			accurate.LabelType:            Equal(accurate.NSTypeRoot),
			constants.OwnerTenant:         Equal("x-team"),
			"foo":                         Equal("bar"),
			accurate.LabelTemplate:        Equal("init-template"),
		}))
		Expect(ns.Annotations).Should(MatchAllKeys(Keys{
			"abc":  Equal("def"),
			"hoge": Equal("fuga"),
		}))

		rb := &rbacv1.RoleBinding{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-x", Name: "x-team-admin"}, rb); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(rb.RoleRef.Name).Should(Equal("admin"))
		Expect(rb.Subjects).Should(ConsistOf([]rbacv1.Subject{
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "x-team",
			},
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "c-team",
			},
		}))

		sw1 := &cattagev1beta1.SyncWindow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sync-window-1",
				Namespace: "app-x",
			},
			Spec: cattagev1beta1.SyncWindowSpec{
				SyncWindows: cattagev1beta1.SyncWindows{
					{
						Kind:         "allow",
						Schedule:     "0 0 * * *",
						Duration:     "1h",
						Applications: []string{"app-c"},
					},
				},
			},
		}
		err = k8sClient.Create(ctx, sw1)
		Expect(err).ToNot(HaveOccurred())

		sw2 := &cattagev1beta1.SyncWindow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sync-window-2",
				Namespace: "sub-4",
			},
			Spec: cattagev1beta1.SyncWindowSpec{
				SyncWindows: cattagev1beta1.SyncWindows{
					{
						Kind:         "deny",
						Schedule:     "0 0 * * *",
						Duration:     "1h",
						Applications: []string{"app-x"},
					},
				},
			},
		}
		err = k8sClient.Create(ctx, sw2)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			sw1 := &cattagev1beta1.SyncWindow{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-x", Name: "sync-window-1"}, sw1); err != nil {
				return err
			}
			if !meta.IsStatusConditionTrue(sw1.Status.Conditions, cattagev1beta1.ConditionSynced) {
				return errors.New("sync-window-1 status condition is not true")
			}

			sw2 := &cattagev1beta1.SyncWindow{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "sub-4", Name: "sync-window-2"}, sw2); err != nil {
				return err
			}
			if !meta.IsStatusConditionTrue(sw2.Status.Conditions, cattagev1beta1.ConditionSynced) {
				return errors.New("sync-window-2 status condition is not true")
			}
			return nil
		}).Should(Succeed())

		proj := argocd.AppProject()
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: "x-team"}, proj); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		Expect(proj.UnstructuredContent()["spec"]).Should(MatchAllKeys(Keys{
			"destinations": ConsistOf(
				MatchAllKeys(Keys{
					"namespace": Equal("app-x"),
					"server":    Equal("*"),
				}),
				MatchAllKeys(Keys{
					"namespace": Equal("sub-4"),
					"server":    Equal("*"),
				}),
				MatchAllKeys(Keys{
					"namespace": Equal("extra-namespace-x"),
					"server":    Equal("*"),
				}),
				MatchAllKeys(Keys{
					"namespace": Equal("extra-namespace-y"),
					"server":    Equal("*"),
				}),
				MatchAllKeys(Keys{
					"namespace": Equal("app-c"),
					"server":    Equal("*"),
				}),
			),
			"namespaceResourceBlacklist": ConsistOf(
				MatchAllKeys(Keys{
					"group": Equal(""),
					"kind":  Equal("ResourceQuota"),
				}),
				MatchAllKeys(Keys{
					"group": Equal(""),
					"kind":  Equal("LimitRange"),
				}),
			),
			"orphanedResources": MatchAllKeys(Keys{
				"warn": Equal(false),
			}),
			"roles": ConsistOf(
				MatchAllKeys(Keys{
					"groups":   ConsistOf("cybozu-go:x-team-gh", "cybozu-go:c-team-gh"),
					"name":     Equal("admin"),
					"policies": ConsistOf("p, proj:x-team:admin, applications, *, x-team/*, allow"),
				}),
			),
			"sourceRepos": ConsistOf("https://github.com/cybozu-go/*"),
			"syncWindows": ConsistOf(
				MatchAllKeys(Keys{
					"kind":       Equal("allow"),
					"schedule":   Equal("0 23 * * *"),
					"duration":   Equal("1h"),
					"namespaces": ConsistOf("*-stage"),
				}),
				MatchAllKeys(Keys{
					"kind":         Equal("allow"),
					"schedule":     Equal("0 0 * * *"),
					"duration":     Equal("1h"),
					"applications": ConsistOf("app-c"),
				}),
				MatchAllKeys(Keys{
					"kind":         Equal("deny"),
					"schedule":     Equal("0 0 * * *"),
					"duration":     Equal("1h"),
					"applications": ConsistOf("app-x"),
				}),
			),
		}))
	})

	It("should create configmaps for sharding", func() {
		allNsCm := &corev1.ConfigMap{}
		defaultCm := &corev1.ConfigMap{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "argocd", Name: "all-tenant-namespaces-cm"}, allNsCm)
			g.Expect(err).NotTo(HaveOccurred())
			allNs := strings.Split(allNsCm.Data["application.namespaces"], ",")
			g.Expect(allNs).Should(ConsistOf("app-x", "sub-4", "app-c"))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "argocd", Name: "default-application-controller-cm"}, defaultCm)
			g.Expect(err).NotTo(HaveOccurred())
			defaultNs := strings.Split(defaultCm.Data["application.namespaces"], ",")
			g.Expect(defaultNs).Should(ConsistOf("app-x", "sub-4", "app-c"))
		}).Should(Succeed())

		tenantS := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-a",
					},
				},
				ControllerName: "second",
			},
		}
		err := k8sClient.Create(ctx, tenantS)
		Expect(err).ToNot(HaveOccurred())

		secondCm := &corev1.ConfigMap{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "argocd", Name: "all-tenant-namespaces-cm"}, allNsCm)
			g.Expect(err).NotTo(HaveOccurred())
			allNs := strings.Split(allNsCm.Data["application.namespaces"], ",")
			g.Expect(allNs).Should(ConsistOf("app-x", "sub-4", "app-a", "sub-1", "sub-2", "sub-3", "app-c"))
		}).Should(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "argocd", Name: "default-application-controller-cm"}, defaultCm)
			g.Expect(err).NotTo(HaveOccurred())
			defaultNs := strings.Split(defaultCm.Data["application.namespaces"], ",")
			g.Expect(defaultNs).Should(ConsistOf("app-x", "sub-4", "app-c"))
		}).Should(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "argocd", Name: "second-application-controller-cm"}, secondCm)
			g.Expect(err).NotTo(HaveOccurred())
			secondNs := strings.Split(secondCm.Data["application.namespaces"], ",")
			g.Expect(secondNs).Should(ConsistOf("app-a", "sub-1", "sub-2", "sub-3"))
		}).Should(Succeed())
	})

	It("should disown root namespace", func() {
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "y-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{Name: "app-y1"},
					{Name: "app-y2"},
				},
				ArgoCD: cattagev1beta1.ArgoCDSpec{},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())

		nsy1 := &corev1.Namespace{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "app-y1"}, nsy1); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(nsy1.Labels).Should(MatchAllKeys(Keys{
			"kubernetes.io/metadata.name": Equal("app-y1"),
			accurate.LabelType:            Equal(accurate.NSTypeRoot),
			constants.OwnerTenant:         Equal("y-team"),
			accurate.LabelTemplate:        Equal("init-template"),
		}))

		nsy2 := &corev1.Namespace{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "app-y2"}, nsy2); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(nsy2.Labels).Should(MatchAllKeys(Keys{
			"kubernetes.io/metadata.name": Equal("app-y2"),
			accurate.LabelType:            Equal(accurate.NSTypeRoot),
			constants.OwnerTenant:         Equal("y-team"),
			accurate.LabelTemplate:        Equal("init-template"),
		}))

		rby1 := &rbacv1.RoleBinding{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-y1", Name: "y-team-admin"}, rby1); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(rby1.RoleRef.Name).Should(Equal("admin"))
		Expect(rby1.Subjects).Should(ConsistOf([]rbacv1.Subject{
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "y-team",
			},
		}))
		rby2 := &rbacv1.RoleBinding{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-y2", Name: "y-team-admin"}, rby2); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(rby2.RoleRef.Name).Should(Equal("admin"))
		Expect(rby2.Subjects).Should(ConsistOf([]rbacv1.Subject{
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "y-team",
			},
		}))

		proj := argocd.AppProject()
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: "y-team"}, proj); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(proj.UnstructuredContent()["spec"]).Should(MatchAllKeys(Keys{
			"destinations": ConsistOf(
				MatchAllKeys(Keys{
					"namespace": Equal("app-y1"),
					"server":    Equal("*"),
				}),
				MatchAllKeys(Keys{
					"namespace": Equal("app-y2"),
					"server":    Equal("*"),
				}),
			),
			"namespaceResourceBlacklist": ConsistOf(
				MatchAllKeys(Keys{
					"group": Equal(""),
					"kind":  Equal("ResourceQuota"),
				}),
				MatchAllKeys(Keys{
					"group": Equal(""),
					"kind":  Equal("LimitRange"),
				}),
			),
			"orphanedResources": MatchAllKeys(Keys{
				"warn": Equal(false),
			}),
			"roles": ConsistOf(
				MatchAllKeys(Keys{
					"groups":   ConsistOf("cybozu-go:y-team"),
					"name":     Equal("admin"),
					"policies": ConsistOf("p, proj:y-team:admin, applications, *, y-team/*, allow"),
				}),
			),
			"sourceRepos": ConsistOf("*"),
			"syncWindows": ConsistOf(
				MatchAllKeys(Keys{
					"kind":       Equal("allow"),
					"schedule":   Equal("0 23 * * *"),
					"duration":   Equal("1h"),
					"namespaces": ConsistOf("*-stage"),
				}),
			),
		}))

		By("removing app-y2")
		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenant.Name}, tenant)
		Expect(err).ToNot(HaveOccurred())
		tenant.Spec.RootNamespaces = []cattagev1beta1.RootNamespaceSpec{
			{Name: "app-y1"},
		}
		err = k8sClient.Update(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "app-y2"}, nsy2)
			if err != nil {
				return err
			}
			if nsy2.Labels[constants.OwnerTenant] != "" {
				return errors.New("owner label still exists")
			}
			return nil
		}).Should(Succeed())
		Expect(nsy2.Labels).Should(MatchAllKeys(Keys{
			"kubernetes.io/metadata.name": Equal("app-y2"),
			accurate.LabelType:            Equal(accurate.NSTypeRoot),
		}))
		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-y2", Name: "y-team-admin"}, rby2)
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return errors.New("rolebinding still exists")
		}).Should(Succeed())

		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: "y-team"}, proj)
			if err != nil {
				return err
			}
			destinations, found, err := unstructured.NestedSlice(proj.UnstructuredContent(), "spec", "destinations")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("destinations not found")
			}
			for _, d := range destinations {
				if d.(map[string]interface{})["namespace"] == "app-y2" {
					return errors.New("destination still exists")
				}
			}
			return nil
		}).Should(Succeed())

		By("removing app-y1")
		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenant.Name}, tenant)
		Expect(err).ToNot(HaveOccurred())
		tenant.Spec.RootNamespaces = []cattagev1beta1.RootNamespaceSpec{}
		err = k8sClient.Update(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("\"y-team\" is invalid: spec.rootNamespaces: Invalid value: 0: spec.rootNamespaces in body should have at least 1 items"))
	})

	It("should remove tenant", func() {
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "z-team",
				Finalizers: []string{constants.Finalizer},
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{Name: "app-z"},
				},
				ArgoCD: cattagev1beta1.ArgoCDSpec{},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())

		ns := &corev1.Namespace{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "app-z"}, ns); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(ns.Labels).Should(MatchAllKeys(Keys{
			"kubernetes.io/metadata.name": Equal("app-z"),
			accurate.LabelType:            Equal(accurate.NSTypeRoot),
			constants.OwnerTenant:         Equal("z-team"),
			accurate.LabelTemplate:        Equal("init-template"),
		}))

		rb := &rbacv1.RoleBinding{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-z", Name: "z-team-admin"}, rb); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(rb.RoleRef.Name).Should(Equal("admin"))
		Expect(rb.Subjects).Should(ConsistOf([]rbacv1.Subject{
			{
				Kind:     "Group",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "z-team",
			},
		}))

		proj := argocd.AppProject()
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: "z-team"}, proj); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(proj.UnstructuredContent()["spec"]).Should(MatchAllKeys(Keys{
			"destinations": ConsistOf(
				MatchAllKeys(Keys{
					"namespace": Equal("app-z"),
					"server":    Equal("*"),
				}),
			),
			"namespaceResourceBlacklist": ConsistOf(
				MatchAllKeys(Keys{
					"group": Equal(""),
					"kind":  Equal("ResourceQuota"),
				}),
				MatchAllKeys(Keys{
					"group": Equal(""),
					"kind":  Equal("LimitRange"),
				}),
			),
			"orphanedResources": MatchAllKeys(Keys{
				"warn": Equal(false),
			}),
			"roles": ConsistOf(
				MatchAllKeys(Keys{
					"groups":   ConsistOf("cybozu-go:z-team"),
					"name":     Equal("admin"),
					"policies": ConsistOf("p, proj:z-team:admin, applications, *, z-team/*, allow"),
				}),
			),
			"sourceRepos": ConsistOf("*"),
			"syncWindows": ConsistOf(
				MatchAllKeys(Keys{
					"kind":       Equal("allow"),
					"schedule":   Equal("0 23 * * *"),
					"duration":   Equal("1h"),
					"namespaces": ConsistOf("*-stage"),
				}),
			),
		}))

		By("removing tenant")
		err = k8sClient.Delete(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "app-z"}, ns)
			if err != nil {
				return err
			}
			if ns.Labels[constants.OwnerTenant] != "" {
				return errors.New("owner label still exists")
			}
			return nil
		}).Should(Succeed())
		Expect(ns.Labels).Should(MatchAllKeys(Keys{
			"kubernetes.io/metadata.name": Equal("app-z"),
			accurate.LabelType:            Equal(accurate.NSTypeRoot),
		}))
		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "app-z", Name: "z-team-admin"}, rb)
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return errors.New("rolebinding still exists")
		}).Should(Succeed())

		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: "z-team"}, proj)
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return errors.New("appproject still exists")
		}).Should(Succeed())

		Eventually(func() error {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "z-team"}, tenant)
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return errors.New("tenant still exists")
		}).Should(Succeed())
	})

	It("should expose custom metrics", func() {
		customMetricsNames := []string{"cattage_tenant_healthy", "cattage_tenant_unhealthy"}
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "m-team",
				Finalizers: []string{constants.Finalizer},
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{Name: "app-m"},
				},
				ArgoCD: cattagev1beta1.ArgoCDSpec{},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			expected := `
			# HELP cattage_tenant_healthy The tenant status about healthy condition
			# TYPE cattage_tenant_healthy gauge
			cattage_tenant_healthy{name="a-team"} 1
			cattage_tenant_healthy{name="c-team"} 1
			cattage_tenant_healthy{name="m-team"} 1
			cattage_tenant_healthy{name="x-team"} 1
			cattage_tenant_healthy{name="y-team"} 1
			# HELP cattage_tenant_unhealthy The tenant status about unhealthy condition
			# TYPE cattage_tenant_unhealthy gauge
			cattage_tenant_unhealthy{name="a-team"} 0
			cattage_tenant_unhealthy{name="c-team"} 0
			cattage_tenant_unhealthy{name="m-team"} 0
			cattage_tenant_unhealthy{name="x-team"} 0
			cattage_tenant_unhealthy{name="y-team"} 0
			`
			expectedReader := strings.NewReader(expected)
			if err := testutil.GatherAndCompare(k8smetrics.Registry, expectedReader, customMetricsNames...); err != nil {
				return err
			}

			return nil
		}).Should(Succeed())

		By("injecting invalid delegates config")
		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenant.Name}, tenant)
		Expect(err).ToNot(HaveOccurred())
		tenant.Spec.Delegates = []cattagev1beta1.DelegateSpec{
			{
				Name: "team-does-not-exist",
				Roles: []string{
					"admin",
				},
			},
		}
		err = k8sClient.Update(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() error {
			expected := `
			# HELP cattage_tenant_healthy The tenant status about healthy condition
			# TYPE cattage_tenant_healthy gauge
			cattage_tenant_healthy{name="a-team"} 1
			cattage_tenant_healthy{name="c-team"} 1
			cattage_tenant_healthy{name="m-team"} 0
			cattage_tenant_healthy{name="x-team"} 1
			cattage_tenant_healthy{name="y-team"} 1
			# HELP cattage_tenant_unhealthy The tenant status about unhealthy condition
			# TYPE cattage_tenant_unhealthy gauge
			cattage_tenant_unhealthy{name="a-team"} 0
			cattage_tenant_unhealthy{name="c-team"} 0
			cattage_tenant_unhealthy{name="m-team"} 1
			cattage_tenant_unhealthy{name="x-team"} 0
			cattage_tenant_unhealthy{name="y-team"} 0
			`
			expectedReader := strings.NewReader(expected)
			if err := testutil.GatherAndCompare(k8smetrics.Registry, expectedReader, customMetricsNames...); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("removing invalid delegates config")
		err = k8sClient.Get(ctx, client.ObjectKey{Name: tenant.Name}, tenant)
		Expect(err).ToNot(HaveOccurred())
		tenant.Spec.Delegates = []cattagev1beta1.DelegateSpec{}
		err = k8sClient.Update(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() error {
			expected := `
			# HELP cattage_tenant_healthy The tenant status about healthy condition
			# TYPE cattage_tenant_healthy gauge
			cattage_tenant_healthy{name="a-team"} 1
			cattage_tenant_healthy{name="c-team"} 1
			cattage_tenant_healthy{name="m-team"} 1
			cattage_tenant_healthy{name="x-team"} 1
			cattage_tenant_healthy{name="y-team"} 1
			# HELP cattage_tenant_unhealthy The tenant status about unhealthy condition
			# TYPE cattage_tenant_unhealthy gauge
			cattage_tenant_unhealthy{name="a-team"} 0
			cattage_tenant_unhealthy{name="c-team"} 0
			cattage_tenant_unhealthy{name="m-team"} 0
			cattage_tenant_unhealthy{name="x-team"} 0
			cattage_tenant_unhealthy{name="y-team"} 0
			`
			expectedReader := strings.NewReader(expected)
			if err := testutil.GatherAndCompare(k8smetrics.Registry, expectedReader, customMetricsNames...); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("removing tenant")
		err = k8sClient.Delete(ctx, tenant)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() error {
			expected := `
			# HELP cattage_tenant_healthy The tenant status about healthy condition
			# TYPE cattage_tenant_healthy gauge
			cattage_tenant_healthy{name="a-team"} 1
			cattage_tenant_healthy{name="c-team"} 1
			cattage_tenant_healthy{name="x-team"} 1
			cattage_tenant_healthy{name="y-team"} 1
			# HELP cattage_tenant_unhealthy The tenant status about unhealthy condition
			# TYPE cattage_tenant_unhealthy gauge
			cattage_tenant_unhealthy{name="a-team"} 0
			cattage_tenant_unhealthy{name="c-team"} 0
			cattage_tenant_unhealthy{name="x-team"} 0
			cattage_tenant_unhealthy{name="y-team"} 0
			`
			expectedReader := strings.NewReader(expected)
			if err := testutil.GatherAndCompare(k8smetrics.Registry, expectedReader, customMetricsNames...); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
	})

	Context("Migration to Argo CD 2.5", func() {
		It("should remove old applications", func() {
			oldApp, err := fillApplication("app", tenantCfg.ArgoCD.Namespace, "a-team")
			Expect(err).ToNot(HaveOccurred())
			oldApp.SetLabels(map[string]string{
				constants.OwnerAppNamespace: "sub-1",
			})
			oldApp.SetFinalizers([]string{
				"resources-finalizer.argocd.argoproj.io",
			})
			err = k8sClient.Create(ctx, oldApp)
			Expect(err).ToNot(HaveOccurred())

			app := argocd.Application()
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: oldApp.GetName()}, app)
				g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
			}).Should(Succeed())
		})

		It("should not remove normal applications", func() {
			normalApp, err := fillApplication("normal", tenantCfg.ArgoCD.Namespace, "default")
			Expect(err).ToNot(HaveOccurred())
			normalApp.SetFinalizers([]string{
				"resources-finalizer.argocd.argoproj.io",
			})
			err = k8sClient.Create(ctx, normalApp)
			Expect(err).ToNot(HaveOccurred())

			app := argocd.Application()
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: tenantCfg.ArgoCD.Namespace, Name: normalApp.GetName()}, app)
				g.Expect(err).ShouldNot(HaveOccurred())
			}).Should(Succeed())
		})
	})
})

func fillApplication(name, namespace, project string) (*unstructured.Unstructured, error) {
	app := argocd.Application()
	app.SetName(name)
	app.SetNamespace(namespace)
	err := unstructured.SetNestedField(app.UnstructuredContent(), project, "spec", "project")
	if err != nil {
		return nil, err
	}
	err = unstructured.SetNestedField(app.UnstructuredContent(), "https://github.com/neco-test/apps-sandbox.git", "spec", "source", "repoURL")
	if err != nil {
		return nil, err
	}
	err = unstructured.SetNestedMap(app.UnstructuredContent(), map[string]interface{}{}, "spec", "destination")
	if err != nil {
		return nil, err
	}

	return app, nil
}
