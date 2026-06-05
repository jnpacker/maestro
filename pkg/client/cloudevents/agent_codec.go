package cloudevents

import (
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventstypes "github.com/cloudevents/sdk-go/v2/types"
	"k8s.io/apimachinery/pkg/runtime"
	workv1 "open-cluster-management.io/api/work/v1"
	workpayload "open-cluster-management.io/sdk-go/pkg/cloudevents/clients/work/payload"
	cetypes "open-cluster-management.io/sdk-go/pkg/cloudevents/generic/types"
)

// AgentCodec wraps the standard OCM ManifestBundle codec and adds an intercept
// path for attested CloudEvents (type prefix io.fleetshift.attestation.manifestbundle.v1).
//
// On Decode: if the event is attested, the VerificationBundle is extracted and
// verified before unwrapping to a standard ManifestWork. Non-attested events
// are handled identically to the wrapped codec.
//
// Wiring: The OCM agent framework selects its codec via the string registry
// (CloudEventsClientCodecs). Until the SDK exposes a hook for custom codecs,
// the AgentCodec is used by intercepting at the subscription layer inside
// Maestro's agent setup. See cmd/maestro/agent/cmd.go for the wiring point.
type AgentCodec struct {
	verifierFactory func(mode string) Verifier
}

// NewAgentCodec creates an AgentCodec. verifierFactory maps an attestation mode
// string to the appropriate Verifier. Pass nil to use VerifierFor (the default).
func NewAgentCodec(verifierFactory func(mode string) Verifier) *AgentCodec {
	if verifierFactory == nil {
		verifierFactory = VerifierFor
	}
	return &AgentCodec{verifierFactory: verifierFactory}
}

// EventDataType returns the attested data type so the agent can register this
// codec alongside the standard ManifestBundle codec.
func (c *AgentCodec) EventDataType() cetypes.CloudEventsDataType {
	return cetypes.CloudEventsDataType{
		Group:    "io.fleetshift.attestation",
		Version:  "v1",
		Resource: "manifestbundle",
	}
}

// Decode processes an attested CloudEvent: verifies the attestation chain and
// returns a ManifestWork built from the verified ManifestBundle.
// Returns an error if the event is not attested, if verification fails, or if
// the bundle cannot be decoded.
func (c *AgentCodec) Decode(evt *cloudevents.Event) (*workv1.ManifestWork, error) {
	if !isAttestedEventType(evt.Type()) {
		return nil, fmt.Errorf("agent_codec: event type %q is not an attested event; use the standard ManifestBundle codec", evt.Type())
	}

	bundle, err := DecodeAttestationManifestBundle(evt)
	if err != nil {
		return nil, fmt.Errorf("agent_codec: failed to decode attestation bundle: %w", err)
	}

	verifier := c.verifierFactory(bundle.AttestationMode)
	if err := verifier.Verify(bundle); err != nil {
		return nil, fmt.Errorf("agent_codec: attestation verification rejected event: %w", err)
	}

	return manifestBundleToWork(evt, bundle)
}

// manifestBundleToWork converts a verified AttestationManifestBundle into the
// ManifestWork that the OCM spoke agent applies to the target cluster.
func manifestBundleToWork(evt *cloudevents.Event, bundle *AttestationManifestBundle) (*workv1.ManifestWork, error) {
	if bundle.ManifestBundle == nil {
		return nil, fmt.Errorf("agent_codec: manifest bundle is nil")
	}

	manifests := make([]workv1.Manifest, 0, len(bundle.ManifestBundle.Manifests))
	for _, raw := range bundle.ManifestBundle.Manifests {
		b, err := raw.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("agent_codec: failed to marshal manifest: %w", err)
		}
		manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: b}})
	}

	ext := evt.Extensions()
	resourceID, _ := cloudeventstypes.ToString(ext[cetypes.ExtensionResourceID])

	work := &workv1.ManifestWork{}
	work.Name = resourceID
	work.Spec.Workload = workv1.ManifestsTemplate{
		Manifests: manifests,
	}

	if bundle.ManifestBundle.DeleteOption != nil {
		work.Spec.DeleteOption = bundle.ManifestBundle.DeleteOption
	}
	if len(bundle.ManifestBundle.ManifestConfigs) > 0 {
		work.Spec.ManifestConfigs = bundle.ManifestBundle.ManifestConfigs
	}

	// Carry the attested data type in an annotation so the agent can log/audit it.
	work.Annotations = map[string]string{
		"attestation.fleetshift.io/mode":       bundle.AttestationMode,
		"attestation.fleetshift.io/event-type": workpayload.ManifestBundleEventDataType.String(),
	}

	return work, nil
}
