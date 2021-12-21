package argocd

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Application() *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: ApplicationVersion,
		Kind:    "Application",
	})
	return app
}

func ApplicationList() *unstructured.UnstructuredList {
	apps := &unstructured.UnstructuredList{}
	apps.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: ApplicationVersion,
		Kind:    "ApplicationList",
	})
	return apps
}
