package hooks

import (
	"context"

	"github.com/cybozu-go/cattage/pkg/argocd"
	"github.com/cybozu-go/cattage/pkg/constants"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

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

var _ = Describe("Application webhook", func() {
	ctx := context.Background()

	It("should allow creating an application on argocd namespace", func() {
		app, err := fillApplication("app-on-argocd", "argocd", "default")
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(app, constants.Finalizer)).To(BeFalse())
	})

	It("should allow creating a normal application", func() {
		app, err := fillApplication("normal-app", "sub-1", "a-team")
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(app, constants.Finalizer)).To(BeTrue())
	})

	It("should deny creating an application on unmanaged namespace", func() {
		app, err := fillApplication("unmanaged-app", "default", "a-team")
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, app)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring(" cannot create the application on a namespace that does not belong to a tenant"))
	})

	It("should deny creating an application managed by other application", func() {
		app, err := fillApplication("other-app", "argocd", "a-team")
		Expect(err).NotTo(HaveOccurred())
		app.SetLabels(map[string]string{
			constants.OwnerAppNamespace: "sub-other",
		})
		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())

		app, err = fillApplication("other-app", "sub-1", "a-team")
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Create(ctx, app)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("the application is already managed by other namespace"))
	})

	It("should allow creating an application managed by nobody", func() {
		app, err := fillApplication("nobody-app", "argocd", "a-team")
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())

		app, err = fillApplication("nobody-app", "sub-1", "a-team")
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow creating an application managed by myself", func() {
		app, err := fillApplication("my-app", "argocd", "a-team")
		Expect(err).NotTo(HaveOccurred())
		app.SetLabels(map[string]string{
			constants.OwnerAppNamespace: "sub-1",
		})
		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())

		app, err = fillApplication("my-app", "sub-1", "a-team")
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny creating an application with other tenant project", func() {
		app, err := fillApplication("unmanaged-app", "sub-1", "b-team")
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, app)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("project of the application does not match the tenant name"))
	})
})
