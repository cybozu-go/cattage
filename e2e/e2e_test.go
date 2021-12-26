package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/cybozu-go/neco-tenant-controller/pkg/argocd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("neco-tenant-controller", func() {
	It("should prepare", func() {
		kubectlSafe(nil, "apply", "-f", "../config/samples/00_template.yaml")
		kubectlSafe(nil, "apply", "-f", "../config/samples/01_tenant.yaml")
		kubectlSafe(nil, "apply", "-f", "../config/samples/02_subnamespace.yaml")
		kubectlSafe(nil, "apply", "-f", "../config/samples/03_application.yaml")
	})

	It("should sync application", func() {
		Eventually(func() error {
			out, err := kubectl(nil, "get", "app", "-n", "sub-1", "sample", "-o", "json")
			if err != nil {
				return err
			}
			app := argocd.Application()
			if err := json.Unmarshal(out, app); err != nil {
				return err
			}
			healthStatus, found, err := unstructured.NestedString(app.UnstructuredContent(), "status", "health", "status")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("status not found")
			}
			if healthStatus != "Healthy" {
				return errors.New("status is not healthy")
			}

			syncStatus, found, err := unstructured.NestedString(app.UnstructuredContent(), "status", "sync", "status")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("status not found")
			}
			if syncStatus != "Synced" {
				return errors.New("status is not synced")
			}

			return nil
		}).Should(Succeed())
	})

	It("should change ownership", func() {
		kubectlSafe(nil, "label", "ns", "sub-1", "accurate.cybozu.com/parent=app-b", "--overwrite")

		Eventually(func() error {
			out, err := kubectl(nil, "get", "app", "-n", "sub-1", "sample", "-o", "json")
			if err != nil {
				return err
			}
			app := argocd.Application()
			if err := json.Unmarshal(out, app); err != nil {
				return err
			}

			project, found, err := unstructured.NestedString(app.UnstructuredContent(), "spec", "project")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("project not found")
			}
			if project != "b-team" {
				return errors.New("project is not fixed")
			}

			healthStatus, found, err := unstructured.NestedString(app.UnstructuredContent(), "status", "health", "status")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("status not found")
			}
			if healthStatus != "Healthy" {
				return errors.New("status is not healthy")
			}

			syncStatus, found, err := unstructured.NestedString(app.UnstructuredContent(), "status", "sync", "status")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("status not found")
			}
			if syncStatus != "Synced" {
				return errors.New("status is not synced")
			}

			return nil
		}).Should(Succeed())
	})
})
