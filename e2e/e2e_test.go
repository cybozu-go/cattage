package e2e

import (
	_ "embed"
	"encoding/json"
	"errors"

	"github.com/cybozu-go/cattage/internal/argocd"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Cattage", func() {
	It("should prepare", func() {
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "../config/samples/template.yaml")
			return err
		}).Should(Succeed())
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "../config/samples/tenant.yaml")
			return err
		}).Should(Succeed())
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "../config/samples/subnamespace.yaml")
			return err
		}).Should(Succeed())
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "../config/samples/application.yaml")
			return err
		}).Should(Succeed())
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
})
