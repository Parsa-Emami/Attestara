// Package verify assesses signatures without accepting or replacing release authority.
package verify

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"unicode/utf8"

	"attestara/internal/contracts"
)

// TrustPolicy binds exact signer identities to independently configured public keys.
// Callers must not mutate the policy or input slices during verification.
type TrustPolicy map[string]ed25519.PublicKey

// SignatureVerification separates mathematical validity from configured identity/key
// membership. SignerTrusted alone does not authenticate the signed manifest.
type SignatureVerification struct {
	CryptographicValid bool
	SignerTrusted      bool
}

// Trusted requires both checks. It authenticates manifest bytes only; callers must
// also validate referenced files with bundle.ReadRoot or bundle.ReadArchive.
func (v SignatureVerification) Trusted() bool { return v.CryptographicValid && v.SignerTrusted }

func signatureError(code, field string) error {
	return &contracts.Error{Code: code, Field: field, Message: "Signature input or configured signer key is invalid or unavailable."}
}
func signatureInterrupted(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return signatureError("deadline_exceeded", "$")
	}
	if ctx.Err() != nil {
		return signatureError("cancelled", "$")
	}
	return nil
}

// VerifySignature verifies a detached Ed25519 signature over exact C08 v1 manifest
// bytes, without reserializing them. An absent signature is unsigned/unverified.
// publicKey is candidate verification material, never a source of trust: only the
// explicit policy can establish signer/key membership. No I/O, execution, network,
// persistence, expiry or revision/replay acceptance occurs here. C04/C15 results
// remain unchanged. Cryptographic calls are bounded but not interruptible mid-call.
// https://pkg.go.dev/crypto/ed25519#Verify
func VerifySignature(parent context.Context, manifest []byte, signerID string, publicKey ed25519.PublicKey, signature []byte, policy TrustPolicy) (SignatureVerification, error) {
	ctx, cancel := context.WithTimeout(parent, contracts.ValidationDeadline)
	defer cancel()
	if _, err := contracts.DecodeInput(ctx, manifest); err != nil {
		return SignatureVerification{}, err
	}
	if len(signature) == 0 {
		return SignatureVerification{}, nil
	}
	if strings.TrimSpace(signerID) == "" || !utf8.ValidString(signerID) || utf8.RuneCountInString(signerID) > 4096 {
		return SignatureVerification{}, signatureError("invalid_input", "signer_id")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return SignatureVerification{}, signatureError("invalid_input", "public_key")
	}
	if len(signature) != ed25519.SignatureSize {
		return SignatureVerification{}, signatureError("invalid_input", "signature")
	}
	trustedKey, known := policy[signerID]
	if known && len(trustedKey) != ed25519.PublicKeySize {
		return SignatureVerification{}, signatureError("invalid_input", "trust_policy")
	}
	result := SignatureVerification{CryptographicValid: ed25519.Verify(publicKey, manifest, signature), SignerTrusted: known && bytes.Equal(publicKey, trustedKey)}
	if err := signatureInterrupted(ctx); err != nil {
		return SignatureVerification{}, err
	}
	return result, nil
}
