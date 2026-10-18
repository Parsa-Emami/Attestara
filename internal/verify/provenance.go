package verify

import (
	"context"
	"errors"

	"attestara/internal/contracts"
	"attestara/internal/evidence"
)

// ProvenanceVerification compares subject claims only, without issuer trust or
// builder/source verification. It does not establish actual file integrity.
type ProvenanceVerification struct {
	SubjectBound bool
}

// VerifyProvenance compares the subject digest in an existing C07 v1 envelope
// with the C08 v1 manifest. A valid mismatch returns SubjectBound=false, nil.
// Callers retain responsibility for C08 file checks and independent C16 trust.
// No I/O, execution, persistence or revision authority occurs here. Callers must
// not mutate input slices during verification. Envelope decoding is bounded but
// synchronous; cancellation is checked before and after that decode.
func VerifyProvenance(parent context.Context, manifest, envelope []byte) (ProvenanceVerification, error) {
	ctx, cancel := context.WithTimeout(parent, contracts.ValidationDeadline)
	defer cancel()
	input, err := contracts.DecodeInput(ctx, manifest)
	if err != nil {
		return ProvenanceVerification{}, err
	}
	provenance, err := evidence.DecodeEnvelope(envelope)
	if err != nil {
		return ProvenanceVerification{}, err
	}
	if ctx.Err() != nil {
		code := "cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = "deadline_exceeded"
		}
		return ProvenanceVerification{}, &contracts.Error{Code: code, Field: "$", Message: "Provenance verification cancelled or deadline exceeded."}
	}
	return ProvenanceVerification{SubjectBound: input.SubjectDigest == provenance.SubjectDigest}, nil
}
