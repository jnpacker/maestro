package cloudevents

import (
	"encoding/json"
	"fmt"
)

// Verifier verifies an AttestationManifestBundle before the agent applies it.
// Implementations are mode-specific: intent mode checks the content hash against
// the delivered manifests; output mode additionally verifies co-signatures.
type Verifier interface {
	Verify(bundle *AttestationManifestBundle) error
}

// NoopVerifier accepts all bundles without verification. Used when the trust
// material has not yet been bootstrapped on the agent, or in tests.
type NoopVerifier struct{}

func (NoopVerifier) Verify(_ *AttestationManifestBundle) error { return nil }

// IntentVerifier verifies that the SHA-256 content hash in the VerificationBundle
// matches the delivered manifests. This proves the hub has not tampered with what
// the user signed.
type IntentVerifier struct{}

func (v IntentVerifier) Verify(bundle *AttestationManifestBundle) error {
	if bundle.ManifestBundle == nil {
		return fmt.Errorf("attestation verification failed: manifest bundle is nil")
	}

	signedInput, ok := bundle.Attestation["signed_input"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("attestation verification failed: signed_input not found in attestation bundle")
	}

	expectedHash, ok := signedInput["content_hash"].(string)
	if !ok || expectedHash == "" {
		return fmt.Errorf("attestation verification failed: content_hash not found in signed_input")
	}

	// Compute hash of the delivered manifests to compare against the signed hash.
	manifestsJSON, err := json.Marshal(bundle.ManifestBundle.Manifests)
	if err != nil {
		return fmt.Errorf("attestation verification failed: cannot marshal manifests for hashing: %v", err)
	}

	actualHash := contentHash(manifestsJSON)
	if actualHash != expectedHash {
		return fmt.Errorf("attestation verification failed: content hash mismatch (signed=%q, delivered=%q)", expectedHash, actualHash)
	}

	return nil
}

// OutputVerifier extends IntentVerifier by also checking addon co-signatures and
// output constraints. The full verification chain is deferred to OME-57; this
// placeholder calls through to IntentVerifier and is extended incrementally.
type OutputVerifier struct {
	IntentVerifier
}

func (v OutputVerifier) Verify(bundle *AttestationManifestBundle) error {
	// Start with intent verification (content hash check).
	if err := v.IntentVerifier.Verify(bundle); err != nil {
		return err
	}
	// TODO(OME-57): verify addon co-signatures and output constraints once trust
	// material bootstrap is implemented.
	return nil
}

// VerifierFor returns the appropriate Verifier for the given attestation mode.
// Unknown modes fall back to NoopVerifier so the agent is not broken by future
// mode additions before it is updated.
func VerifierFor(mode string) Verifier {
	switch mode {
	case AttestationModeIntent:
		return IntentVerifier{}
	case AttestationModeOutput:
		return OutputVerifier{}
	default:
		return NoopVerifier{}
	}
}
