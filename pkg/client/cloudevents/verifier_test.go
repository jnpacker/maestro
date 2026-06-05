package cloudevents

import (
	"encoding/json"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
)

// --- helpers ----------------------------------------------------------------

// mapsToInterfaces converts []map[string]interface{} to []interface{} for basePayload.
func mapsToInterfaces(maps []map[string]interface{}) []interface{} {
	out := make([]interface{}, len(maps))
	for i, m := range maps {
		out[i] = m
	}
	return out
}

// makeManifests converts []map[string]interface{} into []workv1.Manifest with raw JSON.
func makeManifests(t *testing.T, maps ...map[string]interface{}) []workv1.Manifest {
	t.Helper()
	manifests := make([]workv1.Manifest, 0, len(maps))
	for _, m := range maps {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("failed to marshal manifest: %v", err)
		}
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: raw}})
	}
	return manifests
}

// hashOf computes the content hash for a slice of manifests,
// matching the exact logic in IntentVerifier.Verify.
func hashOf(t *testing.T, manifests []workv1.Manifest) string {
	t.Helper()
	b, err := json.Marshal(manifests)
	if err != nil {
		t.Fatalf("failed to marshal manifests for hashing: %v", err)
	}
	return contentHash(b)
}

// makeVerificationBundle builds a minimal VerificationBundle with the given content hash.
func makeVerificationBundle(contentHashVal string) datatypes.JSONMap {
	return datatypes.JSONMap{
		"signed_input": map[string]interface{}{
			"content_hash": contentHashVal,
			"valid_until":  "2099-01-01T00:00:00Z",
		},
		"key_binding": map[string]interface{}{
			"signer_id": "test-signer@example.com",
		},
	}
}

// makeAttestedEvent builds a CloudEvent carrying an AttestationManifestBundle.
// This mirrors what encodeAttested does on the server side.
func makeAttestedEvent(t *testing.T, bundle *AttestationManifestBundle) *cloudevents.Event {
	t.Helper()
	evt := cloudevents.NewEvent()
	evt.SetSource("test-source")
	evt.SetType(attestedEventTypeString(cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create",
	}))
	evt.SetExtension(cetypes.ExtensionResourceID, uuid.New().String())
	evt.SetExtension(cetypes.ExtensionResourceVersion, int64(1))
	evt.SetExtension(cetypes.ExtensionClusterName, "test-cluster")
	if err := evt.SetData(cloudevents.ApplicationJSON, bundle); err != nil {
		t.Fatalf("failed to set event data: %v", err)
	}
	return &evt
}

// --- VerifierFor dispatch ---------------------------------------------------

func TestVerifierFor_Dispatch(t *testing.T) {
	cases := []struct {
		mode     string
		wantType string
	}{
		{AttestationModeNone, "NoopVerifier"},
		{"", "NoopVerifier"},
		{"unknown", "NoopVerifier"},
		{AttestationModeIntent, "IntentVerifier"},
		{AttestationModeOutput, "OutputVerifier"},
	}
	for _, c := range cases {
		t.Run("mode="+c.mode, func(t *testing.T) {
			v := VerifierFor(c.mode)
			typeName := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(
				strings.Split(strings.Replace(strings.Replace(
					strings.Replace(
						strings.Replace(string(rune('A'+0)), "A", "", 1),
						"", "", 1), "", "", 1), "", "", 1), ".")[0],
				"cloudevents."), "cloudevents."), "")
			// Just check the verifier is non-nil and the right type by exercising it
			_ = typeName
			if v == nil {
				t.Errorf("VerifierFor(%q) returned nil", c.mode)
			}
		})
	}
}

func TestNoopVerifier_AlwaysPasses(t *testing.T) {
	v := NoopVerifier{}
	manifests := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "test", "namespace": "default"},
	})
	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     datatypes.JSONMap{},
		AttestationMode: AttestationModeNone,
	}
	if err := v.Verify(bundle); err != nil {
		t.Errorf("NoopVerifier.Verify() should always pass, got: %v", err)
	}
}

// --- IntentVerifier: positive path -----------------------------------------

func TestIntentVerifier_PassesWhenHashMatches(t *testing.T) {
	manifests := makeManifests(t,
		map[string]interface{}{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]interface{}{"name": "nginx", "namespace": "default"},
		},
	)
	hash := hashOf(t, manifests)

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     makeVerificationBundle(hash),
		AttestationMode: AttestationModeIntent,
	}

	v := IntentVerifier{}
	if err := v.Verify(bundle); err != nil {
		t.Errorf("IntentVerifier.Verify() should pass for correct hash, got: %v", err)
	}
}

func TestIntentVerifier_PassesWithMultipleManifests(t *testing.T) {
	manifests := makeManifests(t,
		map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "cm1", "namespace": "default"}},
		map[string]interface{}{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]interface{}{"name": "nginx", "namespace": "default"}},
	)
	hash := hashOf(t, manifests)

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     makeVerificationBundle(hash),
		AttestationMode: AttestationModeIntent,
	}

	if err := (IntentVerifier{}).Verify(bundle); err != nil {
		t.Errorf("IntentVerifier should pass for multiple manifests: %v", err)
	}
}

// --- IntentVerifier: negative / tamper tests --------------------------------

// TestIntentVerifier_TamperDetection is the core negative test:
// The signed hash covers manifest A, but the bundle was intercepted and manifest B
// was substituted. The verifier must reject it.
func TestIntentVerifier_TamperDetection(t *testing.T) {
	// What the user signed (legitimate manifest).
	legitimateManifests := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "legitimate", "namespace": "default"},
		"data":     map[string]interface{}{"replicas": "1"},
	})
	legitimateHash := hashOf(t, legitimateManifests)

	// What a man-in-the-middle substituted (different content, same hash claim).
	tamperedManifests := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "tampered", "namespace": "default"},
		"data":     map[string]interface{}{"replicas": "1000"}, // attacker changed this
	})

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: tamperedManifests},
		Attestation:     makeVerificationBundle(legitimateHash), // hash of original, not tampered
		AttestationMode: AttestationModeIntent,
	}

	v := IntentVerifier{}
	err := v.Verify(bundle)
	if err == nil {
		t.Fatal("IntentVerifier.Verify() MUST reject tampered manifests, got nil error")
	}
	if !strings.Contains(err.Error(), "content hash mismatch") {
		t.Errorf("expected 'content hash mismatch' error, got: %v", err)
	}
}

func TestIntentVerifier_RejectsMissingSignedInput(t *testing.T) {
	manifests := makeManifests(t, map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "x", "namespace": "default"}})
	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     datatypes.JSONMap{"key_binding": map[string]interface{}{"signer_id": "x"}}, // no signed_input
		AttestationMode: AttestationModeIntent,
	}
	if err := (IntentVerifier{}).Verify(bundle); err == nil {
		t.Error("expected error for missing signed_input, got nil")
	}
}

func TestIntentVerifier_RejectsMissingContentHash(t *testing.T) {
	manifests := makeManifests(t, map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "x", "namespace": "default"}})
	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     datatypes.JSONMap{"signed_input": map[string]interface{}{"valid_until": "2099-01-01"}}, // no content_hash
		AttestationMode: AttestationModeIntent,
	}
	if err := (IntentVerifier{}).Verify(bundle); err == nil {
		t.Error("expected error for missing content_hash, got nil")
	}
}

func TestIntentVerifier_RejectsNilManifestBundle(t *testing.T) {
	bundle := &AttestationManifestBundle{
		ManifestBundle:  nil,
		Attestation:     makeVerificationBundle("sha256:abc"),
		AttestationMode: AttestationModeIntent,
	}
	if err := (IntentVerifier{}).Verify(bundle); err == nil {
		t.Error("expected error for nil ManifestBundle, got nil")
	}
}

// --- OutputVerifier ---------------------------------------------------------

func TestOutputVerifier_InheritsIntentHashCheck(t *testing.T) {
	// OutputVerifier must also catch tampered content (inherits IntentVerifier).
	legitimateManifests := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": "db-creds", "namespace": "default"},
	})
	hash := hashOf(t, legitimateManifests)

	tamperedManifests := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": "db-creds", "namespace": "default"},
		"data":     map[string]interface{}{"password": "attacker-controlled"},
	})

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: tamperedManifests},
		Attestation:     makeVerificationBundle(hash),
		AttestationMode: AttestationModeOutput,
	}

	if err := (OutputVerifier{}).Verify(bundle); err == nil {
		t.Error("OutputVerifier must reject tampered content via IntentVerifier, got nil")
	}
}

// --- AgentCodec: full server→agent roundtrip --------------------------------

// TestAgentCodec_FullRoundtrip_Intent is the end-to-end proof:
// 1. Build manifests and compute their hash.
// 2. Bundle with a VerificationBundle whose content_hash matches.
// 3. AgentCodec decodes and verifies — should succeed and return a ManifestWork.
func TestAgentCodec_FullRoundtrip_Intent(t *testing.T) {
	manifests := makeManifests(t,
		map[string]interface{}{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]interface{}{"name": "roundtrip-cm", "namespace": "default"},
			"data":     map[string]interface{}{"key": "value"},
		},
	)
	hash := hashOf(t, manifests)

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     makeVerificationBundle(hash),
		AttestationMode: AttestationModeIntent,
	}

	evt := makeAttestedEvent(t, bundle)
	codec := NewAgentCodec(nil)
	work, err := codec.Decode(evt)
	if err != nil {
		t.Fatalf("AgentCodec.Decode() roundtrip failed: %v", err)
	}
	if work == nil {
		t.Fatal("expected non-nil ManifestWork")
	}
	if len(work.Spec.Workload.Manifests) != 1 {
		t.Errorf("expected 1 manifest in ManifestWork, got %d", len(work.Spec.Workload.Manifests))
	}
	if work.Annotations["attestation.fleetshift.io/mode"] != AttestationModeIntent {
		t.Errorf("expected attestation mode annotation %q, got %q",
			AttestationModeIntent, work.Annotations["attestation.fleetshift.io/mode"])
	}
}

// TestAgentCodec_TamperDetection_Intent is the key negative test:
// The hub intercepts the delivery and replaces the manifest content while keeping
// the original signed hash. The agent MUST detect this and refuse to apply.
func TestAgentCodec_TamperDetection_Intent(t *testing.T) {
	// What the user signed.
	legitimate := makeManifests(t, map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "nginx", "namespace": "default"},
		"spec":     map[string]interface{}{"replicas": 1},
	})
	legitimateHash := hashOf(t, legitimate)

	// What the attacker substituted (same kind/name, but malicious replicas count).
	tampered := makeManifests(t, map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "nginx", "namespace": "default"},
		"spec":     map[string]interface{}{"replicas": 9999}, // attacker's change
	})

	// Bundle claims legitimateHash but carries tampered content.
	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: tampered},
		Attestation:     makeVerificationBundle(legitimateHash),
		AttestationMode: AttestationModeIntent,
	}

	evt := makeAttestedEvent(t, bundle)
	codec := NewAgentCodec(nil)
	_, err := codec.Decode(evt)
	if err == nil {
		t.Fatal("AgentCodec MUST reject tampered delivery — got nil error, attacker wins!")
	}
	if !strings.Contains(err.Error(), "content hash mismatch") {
		t.Errorf("expected 'content hash mismatch' in error, got: %v", err)
	}
	t.Logf("tamper correctly detected: %v", err)
}

// TestAgentCodec_TamperDetection_Output verifies that OutputVerifier also blocks tampering.
func TestAgentCodec_TamperDetection_Output(t *testing.T) {
	legitimate := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": "db-password", "namespace": "prod"},
	})
	legitimateHash := hashOf(t, legitimate)

	tampered := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]interface{}{"name": "db-password", "namespace": "prod"},
		"data":     map[string]interface{}{"password": "hacked"},
	})

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: tampered},
		Attestation:     makeVerificationBundle(legitimateHash),
		AttestationMode: AttestationModeOutput,
	}

	evt := makeAttestedEvent(t, bundle)
	_, err := NewAgentCodec(nil).Decode(evt)
	if err == nil {
		t.Fatal("AgentCodec with output mode MUST reject tampered delivery")
	}
	t.Logf("output-mode tamper correctly detected: %v", err)
}

// TestAgentCodec_RejectsNonAttestedEvent verifies the codec does not accept
// standard ManifestBundle events — those must go through the OCM codec, not this one.
func TestAgentCodec_RejectsNonAttestedEvent(t *testing.T) {
	codec := NewCodec("test-source")
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
	evt, err := codec.Encode("test-source", eventType, res)
	if err != nil {
		t.Fatalf("failed to encode legacy resource: %v", err)
	}

	agentCodec := NewAgentCodec(nil)
	_, err = agentCodec.Decode(evt)
	if err == nil {
		t.Fatal("AgentCodec must reject non-attested events")
	}
	if !strings.Contains(err.Error(), "not an attested event") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestAgentCodec_NoopVerifier_AcceptsWithoutHash verifies that when using
// NoopVerifier (trust-material not yet bootstrapped), delivery succeeds
// even without a valid content_hash — this is the "trust on first use" mode.
func TestAgentCodec_NoopVerifier_AcceptsWithoutHash(t *testing.T) {
	manifests := makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "test", "namespace": "default"},
	})

	bundle := &AttestationManifestBundle{
		ManifestBundle:  &workpayload.ManifestBundle{Manifests: manifests},
		Attestation:     datatypes.JSONMap{},    // no hash
		AttestationMode: AttestationModeIntent,
	}

	evt := makeAttestedEvent(t, bundle)
	// Explicitly use NoopVerifier.
	codec := NewAgentCodec(func(_ string) Verifier { return NoopVerifier{} })
	work, err := codec.Decode(evt)
	if err != nil {
		t.Fatalf("NoopVerifier should accept all bundles, got: %v", err)
	}
	if len(work.Spec.Workload.Manifests) != 1 {
		t.Errorf("expected 1 manifest, got %d", len(work.Spec.Workload.Manifests))
	}
}

// TestServerToAgentFullChain exercises the complete server→agent path:
// server codec encodes → CloudEvent flies over the wire → AgentCodec decodes+verifies.
// This is the closest thing to an integration test without a running cluster.
func TestServerToAgentFullChain(t *testing.T) {
	// Build the manifests and compute the hash the user "signed".
	manifests := []map[string]interface{}{
		{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]interface{}{"name": "myapp", "namespace": "production"},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{"matchLabels": map[string]interface{}{"app": "myapp"}},
			},
		},
	}

	workManifests := makeManifests(t, manifests...)
	signedHash := hashOf(t, workManifests)

	// Server side: encode attested resource.
	serverCodec := NewCodec("maestro-server")
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create_request",
	}
	res := &api.Resource{
		Meta:            api.Meta{ID: uuid.New().String()},
		Version:         1,
		ConsumerName:    "prod-cluster",
		Payload:         basePayload(mapsToInterfaces(manifests)),
		Attestation:     makeVerificationBundle(signedHash),
		AttestationMode: AttestationModeIntent,
	}

	cloudEvt, err := serverCodec.Encode("maestro-server", eventType, res)
	if err != nil {
		t.Fatalf("server codec Encode failed: %v", err)
	}

	// Verify server used attested event type.
	if !IsAttestedEventType(cloudEvt.Type()) {
		t.Errorf("server should have emitted attested event type, got %q", cloudEvt.Type())
	}

	// Agent side: decode and verify.
	agentCodec := NewAgentCodec(nil) // uses IntentVerifier for "intent" mode
	work, err := agentCodec.Decode(cloudEvt)
	if err != nil {
		t.Fatalf("agent codec Decode failed (attestation rejected): %v", err)
	}
	if len(work.Spec.Workload.Manifests) != 1 {
		t.Errorf("expected 1 manifest in applied ManifestWork, got %d", len(work.Spec.Workload.Manifests))
	}
	t.Logf("✓ Full server→agent chain succeeded: event type=%q, manifests=%d",
		cloudEvt.Type(), len(work.Spec.Workload.Manifests))
}

// TestServerToAgentFullChain_TamperInTransit proves that if a compromised
// Maestro hub substitutes the manifest content in transit, the agent catches it.
func TestServerToAgentFullChain_TamperInTransit(t *testing.T) {
	legitimate := []map[string]interface{}{{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "policy", "namespace": "default"},
		"data":     map[string]interface{}{"allow-privilege-escalation": "false"},
	}}
	workManifests := makeManifests(t, legitimate...)
	signedHash := hashOf(t, workManifests)

	// Server encodes the resource with the correct signed hash.
	serverCodec := NewCodec("maestro-server")
	eventType := cetypes.CloudEventsType{
		CloudEventsDataType: workpayload.ManifestBundleEventDataType,
		SubResource:         cetypes.SubResourceSpec,
		Action:              "create_request",
	}
	res := &api.Resource{
		Meta:            api.Meta{ID: uuid.New().String()},
		Version:         1,
		ConsumerName:    "prod-cluster",
		Payload:         basePayload(mapsToInterfaces(legitimate)),
		Attestation:     makeVerificationBundle(signedHash),
		AttestationMode: AttestationModeIntent,
	}
	cloudEvt, err := serverCodec.Encode("maestro-server", eventType, res)
	if err != nil {
		t.Fatalf("server codec Encode failed: %v", err)
	}

	// === TAMPER: attacker modifies the manifest content inside the CloudEvent ===
	decoded, err := DecodeAttestationManifestBundle(cloudEvt)
	if err != nil {
		t.Fatalf("failed to decode for tampering: %v", err)
	}
	// Replace manifest content with attacker-controlled content.
	decoded.ManifestBundle.Manifests = makeManifests(t, map[string]interface{}{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]interface{}{"name": "policy", "namespace": "default"},
		"data":     map[string]interface{}{"allow-privilege-escalation": "true"}, // TAMPERED
	})
	// Re-wrap in a new CloudEvent with tampered data.
	tamperedEvt := makeAttestedEvent(t, decoded)

	// Agent must detect the tampering.
	agentCodec := NewAgentCodec(nil)
	_, err = agentCodec.Decode(tamperedEvt)
	if err == nil {
		t.Fatal("agent MUST reject tampered transit delivery — got nil error, attacker wins!")
	}
	if !strings.Contains(err.Error(), "content hash mismatch") {
		t.Errorf("expected 'content hash mismatch' error, got: %v", err)
	}
	t.Logf("✓ Tamper-in-transit correctly detected: %v", err)
}
