package v1beta1

import (
	"encoding/json"
)

// Params represents untyped configuration.
// kubebuilder does not support interface{} member directly, so this struct is a workaround.
// +kubebuilder:validation:Type=object
type Params struct {
	// Data holds the parameter keys and values.
	Data map[string]interface{} `json:"-"`
}

// ToMap converts the Params to map[string]interface{}. If the receiver is nil, it returns nil.
func (p *Params) ToMap() map[string]interface{} {
	if p == nil {
		return nil
	}
	return p.Data
}

// MarshalJSON implements the Marshaler interface.
func (p *Params) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Data)
}

// UnmarshalJSON implements the Unmarshaler interface.
func (p *Params) UnmarshalJSON(data []byte) error {
	var out map[string]interface{}
	err := json.Unmarshal(data, &out)
	if err != nil {
		return err
	}
	p.Data = out
	return nil
}

// DeepCopyInto is a deep copy function, copying the receiver, writing into `out`. `p` must be non-nil.
func (p *Params) DeepCopyInto(out *Params) {
	bytes, err := json.Marshal(p.Data)
	if err != nil {
		panic(err)
	}
	var clone map[string]interface{}
	err = json.Unmarshal(bytes, &clone)
	if err != nil {
		panic(err)
	}
	out.Data = clone
}
