package presenters

import (
	"encoding/json"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"

	"github.com/openshift-online/maestro/pkg/api"
	"github.com/openshift-online/maestro/pkg/api/openapi"
	"github.com/openshift-online/maestro/pkg/util"
)

// ConvertResourceBundle converts an openapi ResourceBundle request into the domain Resource model.
// It builds a CloudEvent payload from the request's manifests, deleteOption, manifestConfigs, and
// metadata so that the standard ValidateManifestBundle / DecodeManifestBundle path works unchanged.
func ConvertResourceBundle(rb openapi.ResourceBundle) (*api.Resource, error) {
	manifests := make([]workv1.Manifest, 0, len(rb.Manifests))
	for _, m := range rb.Manifests {
		raw, err := json.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest: %v", err)
		}
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: raw}})
	}

	bundle := &workpayload.ManifestBundle{Manifests: manifests}

	if len(rb.DeleteOption) > 0 {
		raw, err := json.Marshal(rb.DeleteOption)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal delete_option: %v", err)
		}
		do := &workv1.DeleteOption{}
		if err := json.Unmarshal(raw, do); err != nil {
			return nil, fmt.Errorf("failed to unmarshal delete_option: %v", err)
		}
		bundle.DeleteOption = do
	}

	for _, mc := range rb.ManifestConfigs {
		raw, err := json.Marshal(mc)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal manifest_config: %v", err)
		}
		cfg := workv1.ManifestConfigOption{}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest_config: %v", err)
		}
		bundle.ManifestConfigs = append(bundle.ManifestConfigs, cfg)
	}

	evt := cloudevents.NewEvent()
	evt.SetID(uuid.New().String())
	evt.SetSource("maestro-rest")
	evt.SetType(workpayload.ManifestBundleEventDataType.String() + ".spec.create_request")
	if err := evt.SetData(cloudevents.ApplicationJSON, bundle); err != nil {
		return nil, fmt.Errorf("failed to set event data: %v", err)
	}

	evtJSON, err := json.Marshal(evt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cloudevent: %v", err)
	}
	payload := datatypes.JSONMap{}
	if err := json.Unmarshal(evtJSON, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cloudevent to jsonmap: %v", err)
	}

	if len(rb.Metadata) > 0 {
		metaJSON, err := json.Marshal(rb.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %v", err)
		}
		payload["workmeta"] = string(metaJSON)
	}

	var attestation datatypes.JSONMap
	if len(rb.Attestation) > 0 {
		attestation = datatypes.JSONMap(rb.Attestation)
	}

	return &api.Resource{
		Meta:            api.Meta{ID: util.NilToEmptyString(rb.Id)},
		Name:            util.NilToEmptyString(rb.Name),
		ConsumerName:    util.NilToEmptyString(rb.ConsumerName),
		Payload:         payload,
		Attestation:     attestation,
		AttestationMode: util.NilToEmptyString(rb.AttestationMode),
	}, nil
}

// PresentResourceBundle converts a resource from the API to the openapi representation.
func PresentResourceBundle(resource *api.Resource) (*openapi.ResourceBundle, error) {
	manifestWrapper, err := api.DecodeManifestBundle(resource.Payload)
	if err != nil {
		return nil, err
	}
	status, err := api.DecodeBundleStatus(resource.Status)
	if err != nil {
		return nil, err
	}

	reference := PresentReference(resource.ID, resource)
	rb := &openapi.ResourceBundle{
		Id:           reference.Id,
		Kind:         reference.Kind,
		Href:         reference.Href,
		Name:         openapi.PtrString(resource.Name),
		ConsumerName: openapi.PtrString(resource.ConsumerName),
		Version:      openapi.PtrInt32(resource.Version),
		CreatedAt:    openapi.PtrTime(resource.CreatedAt),
		UpdatedAt:    openapi.PtrTime(resource.UpdatedAt),
		Status:       status,
	}

	if manifestWrapper != nil {
		rb.Metadata = manifestWrapper.Meta
		rb.Manifests = manifestWrapper.Manifests
		rb.ManifestConfigs = manifestWrapper.ManifestConfigs
		rb.DeleteOption = manifestWrapper.DeleteOption
	}

	// set the deletedAt field if the resource has been marked as deleted
	if !resource.DeletedAt.Time.IsZero() {
		rb.DeletedAt = openapi.PtrTime(resource.DeletedAt.Time)
	}

	if len(resource.Attestation) > 0 {
		rb.Attestation = map[string]interface{}(resource.Attestation)
	}
	if resource.AttestationMode != "" {
		rb.AttestationMode = openapi.PtrString(resource.AttestationMode)
	}

	return rb, nil
}
