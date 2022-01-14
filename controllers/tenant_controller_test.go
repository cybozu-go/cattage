package controllers

import (
	"context"
	_ "embed"
	"errors"
	"time"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/pkg/accurate"
	"github.com/cybozu-go/cattage/pkg/argocd"
	cacheclient "github.com/cybozu-go/cattage/pkg/client"
	tenantconfig "github.com/cybozu-go/cattage/pkg/config"
	"github.com/cybozu-go/cattage/pkg/constants"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed testdata/appprojecttemplate.yaml
var appProjectTemplate string

//go:embed testdata/rolebindingtemplate.yaml
var roleBindingTemplate string

var _ = Describe("Tenant controller", func() {
	ctx := context.Background()
	var stopFunc func()
	var config *tenantconfig.Config

	BeforeEach(func() {
		mgr, err := ctrl.NewManager(k8sCfg, ctrl.Options{
			Scheme:             scheme,
			LeaderElection:     false,
			MetricsBindAddress: "0",
			NewClient:          cacheclient.NewCachingClient,
		})
		Expect(err).ToNot(HaveOccurred())

		config = &tenantconfig.Config{
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
				Namespace:          "argocd",
				AppProjectTemplate: appProjectTemplate,
			},
		}
		tr := NewTenantReconciler(mgr.GetClient(), config)
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
		tenant := &cattagev1beta1.Tenant{
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
				Delegates: []cattagev1beta1.Delegate{
					{
						Name: "c-team",
						Roles: []string{
							"admin",
						},
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
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

		proj := argocd.AppProject()
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: config.ArgoCD.Namespace, Name: "x-team"}, proj); err != nil {
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
					"groups":   ConsistOf("cybozu-go:x-team", "cybozu-go:c-team"),
					"name":     Equal("admin"),
					"policies": ConsistOf("p, proj:x-team:admin, applications, *, x-team/*, allow"),
				}),
			),
			"sourceRepos": ConsistOf("https://github.com/cybozu-go/*"),
		}))
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
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: config.ArgoCD.Namespace, Name: "y-team"}, proj); err != nil {
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
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: config.ArgoCD.Namespace, Name: "y-team"}, proj)
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
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: config.ArgoCD.Namespace, Name: "z-team"}, proj); err != nil {
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
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: config.ArgoCD.Namespace, Name: "z-team"}, proj)
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
})
