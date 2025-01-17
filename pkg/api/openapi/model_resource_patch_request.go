/*
maestro Service API

maestro Service API

API version: 0.0.1
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package openapi

import (
	"encoding/json"
)

// checks if the ResourcePatchRequest type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ResourcePatchRequest{}

// ResourcePatchRequest struct for ResourcePatchRequest
type ResourcePatchRequest struct {
	Manifest map[string]interface{} `json:"manifest,omitempty"`
}

// NewResourcePatchRequest instantiates a new ResourcePatchRequest object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewResourcePatchRequest() *ResourcePatchRequest {
	this := ResourcePatchRequest{}
	return &this
}

// NewResourcePatchRequestWithDefaults instantiates a new ResourcePatchRequest object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewResourcePatchRequestWithDefaults() *ResourcePatchRequest {
	this := ResourcePatchRequest{}
	return &this
}

// GetManifest returns the Manifest field value if set, zero value otherwise.
func (o *ResourcePatchRequest) GetManifest() map[string]interface{} {
	if o == nil || IsNil(o.Manifest) {
		var ret map[string]interface{}
		return ret
	}
	return o.Manifest
}

// GetManifestOk returns a tuple with the Manifest field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ResourcePatchRequest) GetManifestOk() (map[string]interface{}, bool) {
	if o == nil || IsNil(o.Manifest) {
		return map[string]interface{}{}, false
	}
	return o.Manifest, true
}

// HasManifest returns a boolean if a field has been set.
func (o *ResourcePatchRequest) HasManifest() bool {
	if o != nil && !IsNil(o.Manifest) {
		return true
	}

	return false
}

// SetManifest gets a reference to the given map[string]interface{} and assigns it to the Manifest field.
func (o *ResourcePatchRequest) SetManifest(v map[string]interface{}) {
	o.Manifest = v
}

func (o ResourcePatchRequest) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ResourcePatchRequest) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Manifest) {
		toSerialize["manifest"] = o.Manifest
	}
	return toSerialize, nil
}

type NullableResourcePatchRequest struct {
	value *ResourcePatchRequest
	isSet bool
}

func (v NullableResourcePatchRequest) Get() *ResourcePatchRequest {
	return v.value
}

func (v *NullableResourcePatchRequest) Set(val *ResourcePatchRequest) {
	v.value = val
	v.isSet = true
}

func (v NullableResourcePatchRequest) IsSet() bool {
	return v.isSet
}

func (v *NullableResourcePatchRequest) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableResourcePatchRequest(val *ResourcePatchRequest) *NullableResourcePatchRequest {
	return &NullableResourcePatchRequest{value: val, isSet: true}
}

func (v NullableResourcePatchRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableResourcePatchRequest) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
