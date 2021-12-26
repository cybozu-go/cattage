package hooks

import (
	"context"

	tenantv1beta1 "github.com/cybozu-go/neco-tenant-controller/api/v1beta1"
	"github.com/cybozu-go/neco-tenant-controller/pkg/constants"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Tenant webhook", func() {
	ctx := context.Background()

	It("should allow creating a tenant", func() {
		tenant := &tenantv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-team",
			},
			Spec: tenantv1beta1.TenantSpec{
				Namespaces: []tenantv1beta1.NamespaceSpec{
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
		tenant := &tenantv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b-team",
			},
			Spec: tenantv1beta1.TenantSpec{
				Namespaces: []tenantv1beta1.NamespaceSpec{
					{
						Name: "app-y-team",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("deny to specify other owner's namespace"))
	})

	It("should deny creating a tenant with template namespace", func() {
		tenant := &tenantv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "d-team",
			},
			Spec: tenantv1beta1.TenantSpec{
				Namespaces: []tenantv1beta1.NamespaceSpec{
					{
						Name: "template",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("deny to specify a namespace other than root"))
	})

	It("should deny creating a tenant with other group's namespace", func() {
		tenant := &tenantv1beta1.Tenant{
			ObjectMeta: metav1.ObjectMeta{
				Name: "e-team",
			},
			Spec: tenantv1beta1.TenantSpec{
				Namespaces: []tenantv1beta1.NamespaceSpec{
					{
						Name: "sub-2",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, tenant)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("deny to specify a sub namespace"))
	})
})
