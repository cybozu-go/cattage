package config

import (
	_ "embed"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//go:embed testdata/config.yaml
var validData []byte

//go:embed testdata/invalid.yaml
var invalidData []byte

func TestLoad(t *testing.T) {
	c := &Config{}
	err := c.Load(validData)
	if err != nil {
		t.Fatal(err)
	}

	if !cmp.Equal(c.Namespace.CommonLabels, map[string]string{"foo": "bar", "a": "b"}) {
		t.Error("wrong common labels:", cmp.Diff(c.Namespace.CommonLabels, map[string]string{"foo": "bar", "a": "b"}))
	}
	if !cmp.Equal(c.Namespace.CommonAnnotations, map[string]string{"hoge": "fuga", "c": "d"}) {
		t.Error("wrong common annotations:", cmp.Diff(c.Namespace.CommonAnnotations, map[string]string{"hoge": "fuga", "c": "d"}))
	}
	if c.Namespace.RoleBindingTemplate != `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
` {
		t.Error("wrong rolebinding template:", cmp.Diff(c.Namespace.RoleBindingTemplate, `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
`))
	}

	if c.ArgoCD.Namespace != "argo" {
		t.Error("wrong argocd namespace:", cmp.Diff(c.ArgoCD.Namespace, "argo"))
	}
	if c.ArgoCD.AppProjectTemplate != `apiVersion: argoproj.io/v1alpha1
kind: AppProject
` {
		t.Error("wrong appproject template:", cmp.Diff(c.ArgoCD.AppProjectTemplate, `apiVersion: argoproj.io/v1alpha1
kind: AppProject
`))
	}

	c = &Config{}
	err = c.Load(invalidData)
	if err == nil {
		t.Fatal("invalid data are loaded successfully")
	}
	t.Log(err)
}

func TestValidate(t *testing.T) {
	testcases := []struct {
		name    string
		config  *Config
		isValid bool
	}{
		{
			name: "valid config",
			config: &Config{
				Namespace: NamespaceConfig{
					CommonLabels: map[string]string{
						"foo": "bar",
						"a":   "b",
					},
					CommonAnnotations: map[string]string{
						"hoge": "fuga",
						"c":    "d",
					},
					RoleBindingTemplate: "kind: RoleBinding",
				},
				ArgoCD: ArgoCDConfig{
					Namespace:          "argo",
					AppProjectTemplate: "kind: AppProject",
				},
			},
			isValid: true,
		},
		{
			name: "invalid common labels",
			config: &Config{
				Namespace: NamespaceConfig{
					CommonLabels: map[string]string{
						"foo!": "bar",
						"a":    "b/c",
					},
					CommonAnnotations: map[string]string{
						"hoge": "fuga",
						"c":    "d",
					},
					RoleBindingTemplate: "kind: RoleBinding",
				},
				ArgoCD: ArgoCDConfig{
					Namespace:          "argo",
					AppProjectTemplate: "kind: AppProject",
				},
			},
			isValid: false,
		},
		{
			name: "invalid common annotations",
			config: &Config{
				Namespace: NamespaceConfig{
					CommonLabels: map[string]string{
						"foo": "bar",
						"a":   "b",
					},
					CommonAnnotations: map[string]string{
						"!-hoge": "fuga",
						"c":      "d",
					},
					RoleBindingTemplate: "kind: RoleBinding",
				},
				ArgoCD: ArgoCDConfig{
					Namespace:          "argo",
					AppProjectTemplate: "kind: AppProject",
				},
			},
			isValid: false,
		},
		{
			name: "empty rolebinding template",
			config: &Config{
				Namespace: NamespaceConfig{
					CommonLabels: map[string]string{
						"foo": "bar",
						"a":   "b",
					},
					CommonAnnotations: map[string]string{
						"hoge": "fuga",
						"c":    "d",
					},
					RoleBindingTemplate: "",
				},
				ArgoCD: ArgoCDConfig{
					Namespace:          "argo",
					AppProjectTemplate: "kind: AppProject",
				},
			},
			isValid: false,
		},
		{
			name: "invalid namespace",
			config: &Config{
				Namespace: NamespaceConfig{
					CommonLabels: map[string]string{
						"foo": "bar",
						"a":   "b",
					},
					CommonAnnotations: map[string]string{
						"hoge": "fuga",
						"c":    "d",
					},
					RoleBindingTemplate: "kind: RoleBinding",
				},
				ArgoCD: ArgoCDConfig{
					Namespace:          "invalid/argo",
					AppProjectTemplate: "kind: AppProject",
				},
			},
			isValid: false,
		},
		{
			name: "empty appproject template",
			config: &Config{
				Namespace: NamespaceConfig{
					CommonLabels: map[string]string{
						"foo": "bar",
						"a":   "b",
					},
					CommonAnnotations: map[string]string{
						"hoge": "fuga",
						"c":    "d",
					},
					RoleBindingTemplate: "kind: RoleBinding",
				},
				ArgoCD: ArgoCDConfig{
					Namespace:          "argo",
					AppProjectTemplate: "",
				},
			},
			isValid: false,
		},
	}

	for _, testcase := range testcases {
		err := testcase.config.Validate()
		if testcase.isValid && err != nil {
			t.Fatalf("%s: %s", testcase.name, err)
		}
		if !testcase.isValid && err == nil {
			t.Fatalf("%s: invalid data are validated successfully", testcase.name)
		}
	}
}
