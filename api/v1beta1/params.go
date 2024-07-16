package v1beta1

import (
	"encoding/json"
)

// Params represents untyped configuration.
// +kubebuilder:validation:Type=object
type Params struct {
	// Data holds the parameter keys and values.
	Data map[string]interface{} `json:"-"`
}

func (c *Params) ToMap() map[string]interface{} {
	if c == nil {
		return nil
	}
	return c.Data
}

// MarshalJSON implements the Marshaler interface.
func (c *Params) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Data)
}

// UnmarshalJSON implements the Unmarshaler interface.
func (c *Params) UnmarshalJSON(data []byte) error {
	var out map[string]interface{}
	err := json.Unmarshal(data, &out)
	if err != nil {
		return err
	}
	c.Data = out
	return nil
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (c *Params) DeepCopyInto(out *Params) {
	bytes, err := json.Marshal(c.Data)
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
