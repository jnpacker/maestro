package cloudevents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
)

// mqttMaxPacketBytes is the MQTT per-packet size limit enforced by Maestro's source_client.go.
const mqttMaxPacketBytes = 512 * 1024

// basePayload returns a minimal valid ManifestBundle CloudEvent payload with the given manifests.
func basePayload(manifests []interface{}) datatypes.JSONMap {
	return datatypes.JSONMap{
		"specversion":     "1.0",
		"datacontenttype": "application/json",
		"data": map[string]interface{}{
			"manifests": manifests,
		},
	}
}

// configMapManifest returns a simple ConfigMap manifest for use in tests.
func configMapManifest(name, namespace string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}
}

// sampleVerificationBundle returns a minimal VerificationBundle-shaped JSONMap
// that simulates a FleetShift attestation bundle. The "content_hash" field is the
// SHA-256 of the content the user signed — the agent uses this to verify the
// delivered manifest has not been tampered with.
func sampleVerificationBundle(contentHash string) datatypes.JSONMap {
	return datatypes.JSONMap{
		"signed_input": map[string]interface{}{
			"content_hash": contentHash,
			"valid_until":  "2099-01-01T00:00:00Z",
			"output_constraints": []interface{}{
				"namespace == 'default'",
			},
			"expected_generation": 1,
		},
		"key_binding": map[string]interface{}{
			"signer_id":  "user@example.com",
			"public_key": "ed25519:AAAA...",
		},
		"trust_anchor": map[string]interface{}{
			"anchor_id": "tenant-idp",
			"jwks_url":  "https://idp.example.com/.well-known/jwks.json",
		},
	}
}

// TestAttestedModeDetection verifies that isAttestedMode correctly identifies
// which attestation modes trigger the verified delivery path.
func TestAttestedModeDetection(t *testing.T) {
	cases := []struct {
		mode     string
		attested bool
	}{
		{AttestationModeNone, false},
		{"", false},
		{AttestationModeIntent, true},
		{AttestationModeOutput, true},
		{"unknown", false},
	}
	for _, c := range cases {
		got := isAttestedMode(c.mode)
		if got != c.attested {
			t.Errorf("isAttestedMode(%q) = %v, want %v", c.mode, got, c.attested)
		}
	}
}

// TestAttestedEventTypeString verifies that the attested event type string is built correctly.
func TestAttestedEventTypeString(t *testing.T) {
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}
	got := attestedEventTypeString(eventType)
	if !strings.HasPrefix(got, AttestationManifestBundleDataType) {
		t.Errorf("expected event type to start with %q, got %q", AttestationManifestBundleDataType, got)
	}
	if !strings.Contains(got, string(cetypes.SubResourceSpec)) {
		t.Errorf("expected event type to contain sub-resource %q, got %q", cetypes.SubResourceSpec, got)
	}
	if !strings.Contains(got, "create") {
		t.Errorf("expected event type to contain action 'create', got %q", got)
	}
}

// TestEncodeAttested_UsesAttestedDataType verifies that when AttestationMode is "intent"
// or "output", the encoded CloudEvent uses the attested data type rather than the
// standard ManifestBundle data type.
func TestEncodeAttested_UsesAttestedDataType(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}

	for _, mode := range []string{AttestationModeIntent, AttestationModeOutput} {
		t.Run("mode="+mode, func(t *testing.T) {
			res := &api.Resource{
				Meta:            api.Meta{ID: resourceID},
				Version:         1,
				ConsumerName:    "cluster1",
				Payload:         basePayload([]interface{}{configMapManifest("test", "default")}),
				Attestation:     sampleVerificationBundle("sha256:abc123"),
				AttestationMode: mode,
			}

			evt, err := codec.Encode("maestro", eventType, res)
			if err != nil {
				t.Fatalf("unexpected encode error: %v", err)
			}
			if !isAttestedEventType(evt.Type()) {
				t.Errorf("expected attested event type, got %q", evt.Type())
			}
			if !strings.HasPrefix(evt.Type(), AttestationManifestBundleDataType) {
				t.Errorf("expected event type prefix %q, got %q", AttestationManifestBundleDataType, evt.Type())
			}
		})
	}
}

// TestEncodeLegacy_UsesStandardDataType verifies that when AttestationMode is "none"
// or empty, the encoded CloudEvent uses the existing ManifestBundle data type.
// This ensures full backward compatibility for resources submitted without attestation.
func TestEncodeLegacy_UsesStandardDataType(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}

	for _, mode := range []string{AttestationModeNone, ""} {
		t.Run("mode="+mode, func(t *testing.T) {
			res := &api.Resource{
				Meta:            api.Meta{ID: resourceID},
				Version:         1,
				ConsumerName:    "cluster1",
				Payload:         basePayload([]interface{}{configMapManifest("test", "default")}),
				AttestationMode: mode,
			}

			evt, err := codec.Encode("maestro", eventType, res)
			if err != nil {
				t.Fatalf("unexpected encode error: %v", err)
			}
			if isAttestedEventType(evt.Type()) {
				t.Errorf("expected standard (non-attested) event type, got %q", evt.Type())
			}
			if !strings.Contains(evt.Type(), workpayload.ManifestBundleEventDataType.String()) {
				t.Errorf("expected event type to contain %q, got %q", workpayload.ManifestBundleEventDataType, evt.Type())
			}
		})
	}
}

// TestAttestedBundleFaithfulness is the core attestation correctness test:
// it verifies that the VerificationBundle included in the encoded CloudEvent
// is identical to what the user originally supplied — the hub does not alter it.
// This is the fundamental property that lets the agent verify "what the user signed
// is what gets applied".
func TestAttestedBundleFaithfulness(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}

	originalBundle := sampleVerificationBundle("sha256:deadbeef")
	res := &api.Resource{
		Meta:            api.Meta{ID: resourceID},
		Version:         1,
		ConsumerName:    "cluster1",
		Payload:         basePayload([]interface{}{configMapManifest("nginx", "default")}),
		Attestation:     originalBundle,
		AttestationMode: AttestationModeIntent,
	}

	evt, err := codec.Encode("maestro", eventType, res)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	decoded, err := DecodeAttestationManifestBundle(evt)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	// Verify the VerificationBundle is faithfully reproduced.
	originalJSON, err := json.Marshal(originalBundle)
	if err != nil {
		t.Fatalf("failed to marshal original bundle: %v", err)
	}
	decodedJSON, err := json.Marshal(decoded.Attestation)
	if err != nil {
		t.Fatalf("failed to marshal decoded bundle: %v", err)
	}
	if string(originalJSON) != string(decodedJSON) {
		t.Errorf("attestation bundle was modified by the hub:\noriginal: %s\ndecoded:  %s", originalJSON, decodedJSON)
	}

	// Verify the manifest content is also faithfully carried.
	if decoded.ManifestBundle == nil {
		t.Fatal("expected manifest bundle to be non-nil")
	}
	if len(decoded.ManifestBundle.Manifests) != 1 {
		t.Errorf("expected 1 manifest, got %d", len(decoded.ManifestBundle.Manifests))
	}

	// Verify attestation mode is preserved.
	if decoded.AttestationMode != AttestationModeIntent {
		t.Errorf("expected attestation mode %q, got %q", AttestationModeIntent, decoded.AttestationMode)
	}
}

// TestAttestedBundleExtensions verifies that the standard CloudEvent extensions
// (resourceID, resourceVersion, clusterName) are correctly set on attested events.
func TestAttestedBundleExtensions(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	consumerName := "cluster-abc"
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}

	res := &api.Resource{
		Meta:            api.Meta{ID: resourceID},
		Version:         5,
		ConsumerName:    consumerName,
		Payload:         basePayload([]interface{}{}),
		Attestation:     sampleVerificationBundle("sha256:abc"),
		AttestationMode: AttestationModeIntent,
	}

	evt, err := codec.Encode("maestro", eventType, res)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	ext := evt.Extensions()
	if ext[cetypes.ExtensionResourceID] != resourceID {
		t.Errorf("resourceID: expected %q, got %v", resourceID, ext[cetypes.ExtensionResourceID])
	}
	if version, ok := ext[cetypes.ExtensionResourceVersion].(int32); !ok || version != 5 {
		t.Errorf("resourceVersion: expected 5, got %v (type %T)", ext[cetypes.ExtensionResourceVersion], ext[cetypes.ExtensionResourceVersion])
	}
	if ext[cetypes.ExtensionClusterName] != consumerName {
		t.Errorf("clusterName: expected %q, got %v", consumerName, ext[cetypes.ExtensionClusterName])
	}
}

// TestDecodeAttestationManifestBundle_RejectsNonAttestedEvent verifies that
// DecodeAttestationManifestBundle returns an error when given a standard (non-attested)
// CloudEvent, preventing agents from accidentally treating legacy events as attested.
func TestDecodeAttestationManifestBundle_RejectsNonAttestedEvent(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}

	// Encode a legacy (non-attested) resource.
	res := &api.Resource{
		Meta:            api.Meta{ID: resourceID},
		Version:         1,
		ConsumerName:    "cluster1",
		Payload:         basePayload([]interface{}{configMapManifest("test", "default")}),
		AttestationMode: AttestationModeNone,
	}
	evt, err := codec.Encode("maestro", eventType, res)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}

	// DecodeAttestationManifestBundle should reject the legacy event.
	_, err = DecodeAttestationManifestBundle(evt)
	if err == nil {
		t.Error("expected error when decoding legacy event as attested, got nil")
	}
	if !strings.Contains(err.Error(), "not an attested manifest bundle event") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestAttestationBundleSize_WithinMQTTLimit verifies that a typical attested event
// stays well within the MQTT 512KB per-packet limit enforced by source_client.go.
func TestAttestationBundleSize_WithinMQTTLimit(t *testing.T) {
	// Construct a realistic-sized bundle: one manifest + a verification bundle.
	bundle := &AttestationManifestBundle{
		ManifestBundle: &workpayload.ManifestBundle{},
		Attestation:    sampleVerificationBundle("sha256:" + strings.Repeat("a", 64)),
		AttestationMode: AttestationModeIntent,
	}

	size, err := AttestationBundleSize(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size > mqttMaxPacketBytes {
		t.Errorf("attestation bundle size %d bytes exceeds MQTT limit of %d bytes", size, mqttMaxPacketBytes)
	}
	t.Logf("attestation bundle size: %d bytes (limit: %d bytes)", size, mqttMaxPacketBytes)
}

// TestAttestationBundleSize_LargeChain verifies that even a 10-hop derivation chain
// (the maximum depth enforced per FM-84 design) stays within the MQTT limit.
func TestAttestationBundleSize_LargeChain(t *testing.T) {
	// Simulate a deep derivation chain with 10 update attestations.
	priorInputs := make([]interface{}, 10)
	for i := range priorInputs {
		priorInputs[i] = map[string]interface{}{
			"content_hash":        "sha256:" + strings.Repeat("b", 64),
			"expected_generation": i + 1,
			"valid_until":         "2099-01-01T00:00:00Z",
		}
	}

	bigBundle := &AttestationManifestBundle{
		ManifestBundle: &workpayload.ManifestBundle{},
		Attestation: datatypes.JSONMap{
			"signed_input":  sampleVerificationBundle("sha256:" + strings.Repeat("c", 64)),
			"prior_inputs":  priorInputs,
			"update_attestations": []interface{}{
				map[string]interface{}{"patch": "update-1", "signature": strings.Repeat("d", 88)},
				map[string]interface{}{"patch": "update-2", "signature": strings.Repeat("d", 88)},
			},
		},
		AttestationMode: AttestationModeOutput,
	}

	size, err := AttestationBundleSize(bigBundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("10-hop derivation chain bundle size: %d bytes (limit: %d bytes)", size, mqttMaxPacketBytes)
	if size > mqttMaxPacketBytes {
		t.Errorf("10-hop chain bundle (%d bytes) exceeds MQTT limit of %d bytes — enforce max_derivation_depth at submission", size, mqttMaxPacketBytes)
	}
}

// TestIsAttestedEventType verifies the event type detection helper.
func TestIsAttestedEventType(t *testing.T) {
	cases := []struct {
		evtType  string
		expected bool
	}{
		{AttestationManifestBundleDataType + ".spec.create", true},
		{AttestationManifestBundleDataType + ".spec.update", true},
		{"io.open-cluster-management.works.v1alpha1.manifestbundles.spec.create", false},
		{"io.open-cluster-management.works.v1alpha1.manifestbundles.status.update_request", false},
		{"", false},
	}
	for _, c := range cases {
		got := isAttestedEventType(c.evtType)
		if got != c.expected {
			t.Errorf("isAttestedEventType(%q) = %v, want %v", c.evtType, got, c.expected)
		}
	}
}

// TestCodecEncode_AttestedVsLegacy is a table-driven test covering the full
// dispatch logic: attested resources use the attested path, legacy resources
// use the standard path, and both carry correct CloudEvent extensions.
func TestCodecEncode_AttestedVsLegacy(t *testing.T) {
	codec := NewCodec("maestro")
	resourceID := uuid.New().String()
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}
	bundle := sampleVerificationBundle("sha256:test")

	cases := []struct {
		name            string
		attestationMode string
		attestation     datatypes.JSONMap
		expectAttested  bool
	}{
		{"no attestation mode", "", nil, false},
		{"mode=none", AttestationModeNone, nil, false},
		{"mode=intent no bundle", AttestationModeIntent, nil, true},
		{"mode=intent with bundle", AttestationModeIntent, bundle, true},
		{"mode=output with bundle", AttestationModeOutput, bundle, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := &api.Resource{
				Meta:            api.Meta{ID: resourceID},
				Version:         1,
				ConsumerName:    "cluster1",
				Payload:         basePayload([]interface{}{configMapManifest("test", "default")}),
				Attestation:     c.attestation,
				AttestationMode: c.attestationMode,
			}

			evt, err := codec.Encode("maestro", eventType, res)
			if err != nil {
				t.Fatalf("unexpected encode error: %v", err)
			}

			gotAttested := isAttestedEventType(evt.Type())
			if gotAttested != c.expectAttested {
				t.Errorf("isAttestedEventType=%v, want %v (event type: %q)", gotAttested, c.expectAttested, evt.Type())
			}

			// All paths must set the standard extensions.
			ext := evt.Extensions()
			if ext[cetypes.ExtensionResourceID] != resourceID {
				t.Errorf("resourceID missing or wrong")
			}
		})
	}
}
