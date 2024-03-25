package hooks

import (
	"context"

	"github.com/cybozu-go/cattage/pkg/argocd"
	"github.com/cybozu-go/cattage/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
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

	It("should allow creating an application in any namespace", func() {
		app, err := fillApplication("tenant", "sub-1", "team-a")
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, app)
		Expect(err).NotTo(HaveOccurred())

		Expect(controllerutil.ContainsFinalizer(app, constants.Finalizer)).To(BeFalse())
	})

	It("should deny creating an application in argocd namespace", func() {
		app, err := fillApplication("default-app", "argocd", "default")
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Create(ctx, app)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).Should(ContainSubstring("cannot create Application in argocd namespace"))
	})
})
