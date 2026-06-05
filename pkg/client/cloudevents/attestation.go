package cloudevents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"gorm.io/datatypes"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"

	"github.com/openshift-online/maestro/pkg/api"
)

const (
	// AttestationManifestBundleDataType is the CloudEvents data type for FleetShift attested deliveries.
	// Agents branch on this type to select the attestation verification path over the legacy apply path.
	AttestationManifestBundleDataType = "io.fleetshift.attestation.manifestbundle.v1"

	// AttestationModeNone is the default — no attestation; agents apply unconditionally (legacy path).
	AttestationModeNone = "none"
	// AttestationModeIntent verifies the user's signed intent (content + constraints) before apply.
	AttestationModeIntent = "intent"
	// AttestationModeOutput verifies the full output chain (addon co-signatures + output constraints).
	AttestationModeOutput = "output"
)

// AttestationManifestBundle is the CloudEvent data payload for attested deliveries.
// It carries the ManifestBundle alongside the self-contained VerificationBundle so
// that the agent can verify the attestation chain locally before calling apply.
type AttestationManifestBundle struct {
	ManifestBundle  *workpayload.ManifestBundle `json:"manifest_bundle"`
	Attestation     datatypes.JSONMap            `json:"attestation"`
	AttestationMode string                       `json:"attestation_mode"`
}

// isAttestedMode returns true when the resource should use the attested delivery path.
func isAttestedMode(mode string) bool {
	return mode == AttestationModeIntent || mode == AttestationModeOutput
}

// IsAttestedEventType returns true when the CloudEvent type uses the attested data type.
func IsAttestedEventType(evtType string) bool {
	return strings.HasPrefix(evtType, AttestationManifestBundleDataType)
}

// isAttestedEventType is the package-internal alias used by the codec and agent_codec.
func isAttestedEventType(evtType string) bool { return IsAttestedEventType(evtType) }

// attestedEventTypeString converts a standard CloudEventsType into the attested variant
// by replacing the data type prefix with AttestationManifestBundleDataType.
func attestedEventTypeString(eventType cetypes.CloudEventsType) string {
	return fmt.Sprintf("%s.%s.%s", AttestationManifestBundleDataType, eventType.SubResource, eventType.Action)
}

// encodeAttested builds a CloudEvent for an attested resource, embedding both the
// ManifestBundle and the VerificationBundle in the event data. The hub stores the
// attestation opaquely and does not perform cryptographic verification.
func encodeAttested(source string, eventType cetypes.CloudEventsType, res *api.Resource) (*cloudevents.Event, error) {
	payloadEvt, err := api.JSONMAPToCloudEvent(res.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to convert resource payload to cloudevent: %v", err)
	}

	manifestBundle := &workpayload.ManifestBundle{}
	if err := payloadEvt.DataAs(manifestBundle); err != nil {
		return nil, fmt.Errorf("failed to decode manifest bundle from payload: %v", err)
	}

	bundle := &AttestationManifestBundle{
		ManifestBundle:  manifestBundle,
		Attestation:     res.Attestation,
		AttestationMode: res.AttestationMode,
	}

	evt := cloudevents.NewEvent()
	evt.SetSource(source)
	evt.SetType(attestedEventTypeString(eventType))
	evt.SetExtension(cetypes.ExtensionResourceID, res.ID)
	evt.SetExtension(cetypes.ExtensionResourceVersion, int64(res.Version))
	evt.SetExtension(cetypes.ExtensionClusterName, res.ConsumerName)

	if err := evt.SetData(cloudevents.ApplicationJSON, bundle); err != nil {
		return nil, fmt.Errorf("failed to set attestation bundle data: %v", err)
	}

	return &evt, nil
}

// DecodeAttestationManifestBundle extracts the AttestationManifestBundle from a CloudEvent.
// Returns an error when the event does not carry the attested data type.
func DecodeAttestationManifestBundle(evt *cloudevents.Event) (*AttestationManifestBundle, error) {
	if !isAttestedEventType(evt.Type()) {
		return nil, fmt.Errorf("event type %q is not an attested manifest bundle event", evt.Type())
	}
	bundle := &AttestationManifestBundle{}
	if err := evt.DataAs(bundle); err != nil {
		return nil, fmt.Errorf("failed to decode attestation manifest bundle: %v", err)
	}
	return bundle, nil
}

// contentHash returns the hex-encoded SHA-256 of the given bytes, prefixed with "sha256:".
// Used by IntentVerifier to compare the signed content hash against delivered manifests.
func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// AttestationBundleSize returns the serialized byte size of the attestation bundle.
// Used to enforce the MQTT 512KB per-packet limit before publishing.
func AttestationBundleSize(bundle *AttestationManifestBundle) (int, error) {
	b, err := json.Marshal(bundle)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal attestation bundle: %v", err)
	}
	return len(b), nil
}
