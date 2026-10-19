// Package report builds deterministic, unsigned release passports from validated
// release inputs and policy results. It does not establish file integrity or trust.
package report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"slices"
	"strings"

	"attestara/internal/contracts"
)

// Passport is a portable projection of a release and its policy result. Its
// digest covers every field except ReportDigest itself.
type Passport struct {
	SchemaVersion      string                        `json:"schema_version"`
	Status             string                        `json:"status"`
	ReleaseID          string                        `json:"release_id"`
	SourceRevision     string                        `json:"source_revision"`
	SubjectDigest      string                        `json:"subject_digest"`
	EvidenceDigest     string                        `json:"evidence_digest"`
	PolicyDigest       string                        `json:"policy_digest"`
	PolicyDecision     string                        `json:"policy_decision"`
	Verification       string                        `json:"verification"`
	EvidenceReferences []contracts.EvidenceReference `json:"evidence_references"`
	Warnings           []string                      `json:"warnings"`
	ReportDigest       string                        `json:"report_digest,omitzero"`
}

func passportError(code, field, message string) error {
	return &contracts.Error{Code: code, Field: field, Message: message}
}

// GeneratePassport returns a deterministic passport for a validated C04 input
// and C18 result. It sorts evidence references by path and digest, preserves
// decision warnings and verification status, and never upgrades trust. Input and
// result are copied through their existing validators; referenced bytes still
// require the C08 reader and signature trust remains an independent C16 result.
func GeneratePassport(parent context.Context, input contracts.Input, result contracts.Result) (Passport, error) {
	ctx, cancel := context.WithTimeout(parent, contracts.ValidationDeadline)
	defer cancel()
	fail := func(code, field string) (Passport, error) {
		return Passport{}, passportError(code, field, "Passport input is invalid or unavailable.")
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fail("deadline_exceeded", "$")
	}
	if ctx.Err() != nil {
		return fail("cancelled", "$")
	}

	inputJSON, err := json.Marshal(input, json.FormatNilSliceAsNull(true))
	if err != nil {
		return fail("invalid_input", "input")
	}
	validatedInput, err := contracts.DecodeInput(ctx, inputJSON)
	if err != nil {
		return Passport{}, err
	}
	resultJSON, err := json.Marshal(result, json.FormatNilSliceAsNull(true))
	if err != nil {
		return fail("invalid_input", "result")
	}
	validatedResult, err := contracts.DecodeResult(ctx, resultJSON)
	if err != nil {
		return Passport{}, err
	}
	if validatedResult.SchemaVersion != "2" {
		return fail("unsupported_version", "result.schema_version")
	}
	if validatedResult.Status == "failed" {
		return Passport{}, passportError(validatedResult.Error.Code, "$", "Release evaluation failed; passport was not generated.")
	}
	outcome := validatedResult.Result
	if outcome.SubjectDigest != validatedInput.SubjectDigest || outcome.EvidenceDigest != validatedInput.EvidenceDigest || outcome.PolicyDigest != validatedInput.PolicyDigest {
		return fail("invalid_input", "result.binding")
	}
	files := make(map[string]string, len(validatedInput.Files))
	for _, file := range validatedInput.Files {
		files[file.Path] = file.Digest
	}
	for _, ref := range outcome.EvidenceReferences {
		if files[ref.Path] != ref.Digest {
			return fail("invalid_input", "result.evidence_references")
		}
	}

	refs := slices.Clone(outcome.EvidenceReferences)
	slices.SortFunc(refs, func(a, b contracts.EvidenceReference) int {
		if order := strings.Compare(a.Path, b.Path); order != 0 {
			return order
		}
		return strings.Compare(a.Digest, b.Digest)
	})
	passport := Passport{
		SchemaVersion:      "1",
		Status:             validatedResult.Status,
		ReleaseID:          validatedInput.ReleaseID,
		SourceRevision:     validatedInput.SourceRevision,
		SubjectDigest:      outcome.SubjectDigest,
		EvidenceDigest:     outcome.EvidenceDigest,
		PolicyDigest:       outcome.PolicyDigest,
		PolicyDecision:     outcome.PolicyDecision,
		Verification:       outcome.Verification,
		EvidenceReferences: refs,
		Warnings:           slices.Clone(validatedResult.Warnings),
	}
	passport = redactPassport(passport)
	payload, err := json.Marshal(passport, json.FormatNilSliceAsNull(true))
	if err != nil {
		return fail("invalid_input", "passport")
	}
	digest := sha256.Sum256(payload)
	passport.ReportDigest = "sha256:" + hex.EncodeToString(digest[:])
	return passport, nil
}

// MatchesInput compares an already validated passport with a validated manifest,
// including the existing public metadata redaction. It does not verify a policy.
func MatchesInput(p Passport, input contracts.Input) bool {
	if p.SubjectDigest != input.SubjectDigest || p.EvidenceDigest != input.EvidenceDigest || p.PolicyDigest != input.PolicyDigest || p.ReleaseID != redactPublicText(input.ReleaseID) || p.SourceRevision != redactPublicText(input.SourceRevision) {
		return false
	}
	for _, ref := range p.EvidenceReferences {
		found := false
		for _, f := range input.Files {
			if ref.Path == f.Path && ref.Digest == f.Digest && ref.Digest == input.EvidenceDigest {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
