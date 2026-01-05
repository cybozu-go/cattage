package client

import (
	"bytes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

func ExtractManagedFields(u *unstructured.Unstructured, manager string) (map[string]interface{}, error) {
	fieldset := &fieldpath.Set{}
	objManagedFields := u.GetManagedFields()
	for _, mf := range objManagedFields {
		if mf.Manager != manager || mf.Operation != metav1.ManagedFieldsOperationApply {
			continue
		}
		fs := &fieldpath.Set{}
		err := fs.FromJSON(bytes.NewReader(mf.FieldsV1.Raw))
		if err != nil {
			return nil, err
		}
		fieldset = fieldset.Union(fs)
	}

	d, err := typed.DeducedParseableType.FromUnstructured(u.Object)
	if err != nil {
		return nil, err
	}

	x := d.ExtractItems(fieldset.Leaves()).AsValue().Unstructured()
	managed, ok := x.(map[string]interface{})
	if !ok {
		managed = make(map[string]interface{})
	}

	managed["apiVersion"] = u.GetAPIVersion()
	managed["kind"] = u.GetKind()
	metadata, ok := managed["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
	}
	metadata["name"] = u.GetName()
	metadata["namespace"] = u.GetNamespace()
	managed["metadata"] = metadata
	return managed, nil
}
