package hooks

import (
	"context"

	cattagev1beta1 "github.com/cybozu-go/cattage/api/v1beta1"
	"github.com/cybozu-go/cattage/pkg/constants"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Tenant webhook", func() {
	ctx := context.Background()

	It("should allow creating a tenant", func() {
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-new",
					},
					{
						Name: "app-a-team",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(tenant, constants.Finalizer)).To(BeTrue())
	})

	It("should deny creating a tenant with other owner's namespace", func() {
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-y-team",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("other owner's namespace is not allowed"))
	})

	It("should deny creating a tenant with template namespace", func() {
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "d-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "template",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("namespace other than root is not allowed"))
	})

	It("should deny creating a tenant with other group's namespace", func() {
		tenant := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "e-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "sub-2",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("sub namespace is not allowed"))
	})

	It("should deny creating a tenant with other tenant's root namespace", func() {
		tenantF := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "f-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-f-team",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenantF)
		Expect(err).NotTo(HaveOccurred())

		tenantG := &cattagev1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "g-team",
			},
			Spec: cattagev1beta1.TenantSpec{
				RootNamespaces: []cattagev1beta1.RootNamespaceSpec{
					{
						Name: "app-f-team",
					},
				},
			},
		}
		err = k8sClient.Create(ctx, tenantG)
		Expect(err).To(HaveOccurred())

		Expect(err.Error()).Should(ContainSubstring("other tenant's root namespace is not allowed"))
	})
})
