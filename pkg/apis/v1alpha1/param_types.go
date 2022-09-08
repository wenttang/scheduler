package v1alpha1

import (
	"encoding/json"
	"fmt"
)

// ParamSpec defines arbitrary parameters needed beyond typed inputs.
type ParamSpec struct {
	// Name declares the name by which a parameter is referenced.
	Name string `json:"name"`

	// Default is the value a parameter takes if no input value is supplied.
	// +optional
	Default *AnyString `json:"default,omitempty"`

	// Type is data type, can be string、int、float and bool default string.
	Type string `json:"type,omitempty"`

	// Description is a user-facing description of the parameter that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`
}

// KeyAndValue declares to use for the value called name.
type KeyAndValue struct {
	Name  string     `json:"name"`
	Value *AnyString `json:"value"`
}

type AnyString string

func ParseAnyString(val interface{}) AnyString {
	switch val := val.(type) {
	case string:
		return AnyString(val)
	case int, int8, int16, int32, int64, float32, float64, bool,
		*int, *int8, *int16, *int32, *int64, *float32, *float64, *bool:
		return AnyString(fmt.Sprintf("%v", val))
	default:
		body, _ := json.Marshal(val)
		return AnyString(string(body))
	}
}

func ParseAnyStringPtr(val interface{}) *AnyString {
	anyString := ParseAnyString(val)
	return &anyString
}

func (anyString *AnyString) GetValue() interface{} {
	if anyString == nil {
		return nil
	}

	var v interface{}
	err := json.Unmarshal([]byte(*anyString), &v)
	if err != nil {
		return anyString.String()
	}
	return v
}

func (anyString *AnyString) GetString() *string {
	if anyString != nil {
		str := string(*anyString)
		return &str
	}
	return nil
}

func (anyString *AnyString) String() string {
	if anyString != nil {
		return string(*anyString)
	}
	return ""
}

func (anyString *AnyString) Float32() (*float32, error) {
	v := anyString.GetValue()

	val, ok := v.(float32)
	if ok {
		return &val, nil
	}

	return nil, fmt.Errorf("expect float32, actual not (%v)", v)
}

func (anyString *AnyString) Float64() (*float64, error) {
	v := anyString.GetValue()

	val, ok := v.(float64)
	if ok {
		return &val, nil
	}

	return nil, fmt.Errorf("expect float64, actual not (%v)", v)
}

func (anyString *AnyString) Int64() (*int64, error) {
	v := anyString.GetValue()

	val, ok := v.(int64)
	if ok {
		return &val, nil
	}

	return nil, fmt.Errorf("expect int64, actual not (%v)", v)
}

func (anyString *AnyString) Int32() (*int32, error) {
	v := anyString.GetValue()

	val, ok := v.(int32)
	if ok {
		return &val, nil
	}

	return nil, fmt.Errorf("expect int32, actual not (%v)", v)
}

func (anyString *AnyString) GetSlice() ([]interface{}, error) {
	v := anyString.GetValue()

	val, ok := v.([]interface{})
	if ok {
		return val, nil
	}

	return nil, fmt.Errorf("expect []interface{}, actual not (%v)", v)
}

func (anyString *AnyString) GetMap() (map[string]interface{}, error) {
	v := anyString.GetValue()

	val, ok := v.(map[string]interface{})
	if ok {
		return val, nil
	}

	return nil, fmt.Errorf("expect map[string]interface{}, actual not (%v)", v)
}
