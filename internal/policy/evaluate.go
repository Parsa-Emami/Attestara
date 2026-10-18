package policy

import (
	"context"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"fmt"

	"attestara/internal/contracts"
)

// Evaluate applies the single first-party v1 evidence_present predicate. The
// evidence JSON supplies a versioned first-party fact, not scanner execution or trust.
// Evidence bytes must match the input's root evidence artifact. Nil bytes mean
// unavailable evidence; a non-nil empty slice represents an actual empty file.
// Subject bytes and durable admission are checked by the storage/bundle port.
// No paths are opened and neither rule nor artifact text is executed.
func Evaluate(parent context.Context, inputJSON, ruleJSON, evidenceBytes []byte) (contracts.Result, error) {
	ctx, cancel := context.WithTimeout(parent, contracts.ValidationDeadline)
	defer cancel()
	fail := func(code, field string) (contracts.Result, error) {
		return contracts.Result{}, &contracts.Error{Code: code, Field: field, Message: "Release rule or evidence input is invalid or unavailable."}
	}
	if ctx.Err() != nil {
		return evaluationInterrupted(ctx)
	}
	if len(ruleJSON) > 4096 {
		return fail("limit_exceeded", "policy")
	}
	var rule struct {
		SchemaVersion string `json:"schema_version"`
		Predicate     string `json:"predicate"`
	}
	// Go JSON v2 rejects duplicate keys/invalid UTF-8; only the fixed predicate
	// is accepted. Source: https://pkg.go.dev/encoding/json/v2#Unmarshal
	if err := json.Unmarshal(ruleJSON, &rule, json.RejectUnknownMembers(true)); err != nil {
		var semantic *json.SemanticError
		if errors.As(err, &semantic) {
			return fail("invalid_input", "policy")
		}
		return fail("invalid_json", "policy")
	}
	if rule.SchemaVersion == "" {
		return fail("invalid_input", "policy.schema_version")
	}
	if rule.SchemaVersion != "1" {
		return fail("unsupported_version", "policy.schema_version")
	}
	if rule.Predicate != "evidence_present" {
		return fail("invalid_input", "policy.predicate")
	}
	input, err := contracts.DecodeInput(ctx, inputJSON)
	if err != nil {
		return contracts.Result{}, err
	}
	if policyHash(ruleJSON) != input.PolicyDigest {
		return fail("invalid_input", "policy_digest")
	}
	refs := make([]contracts.EvidenceReference, 0)
	var facts struct {
		SchemaVersion   string `json:"schema_version"`
		EvidencePresent *bool  `json:"evidence_present"`
	}
	if evidenceBytes != nil {
		if int64(len(evidenceBytes)) > contracts.MaxBundleBytes {
			return fail("limit_exceeded", "evidence")
		}
		if policyHash(evidenceBytes) != input.EvidenceDigest {
			return fail("invalid_input", "evidence_digest")
		}
		if err := json.Unmarshal(evidenceBytes, &facts, json.RejectUnknownMembers(true)); err != nil {
			var semantic *json.SemanticError
			if errors.As(err, &semantic) {
				return fail("invalid_input", "evidence")
			}
			return fail("invalid_json", "evidence")
		}
		if facts.SchemaVersion == "" {
			return fail("invalid_input", "evidence.schema_version")
		}
		if facts.SchemaVersion != "1" {
			return fail("unsupported_version", "evidence.schema_version")
		}
		for _, file := range input.Files {
			if file.Digest == input.EvidenceDigest {
				if *file.SizeBytes != int64(len(evidenceBytes)) {
					return fail("invalid_input", "files.size_bytes")
				}
				refs = append(refs, contracts.EvidenceReference{Path: file.Path, Digest: file.Digest})
			}
		}
	}
	decision := "unknown"
	if len(refs) > 0 && facts.EvidencePresent != nil {
		decision = "fail"
		if *facts.EvidencePresent {
			decision = "pass"
		}
	}
	if ctx.Err() != nil {
		return evaluationInterrupted(ctx)
	}
	return contracts.Result{SchemaVersion: "2", Status: "unverified", InputDigest: policyHash(inputJSON), EngineVersion: "0.0.1", Warnings: []string{}, Result: &contracts.Outcome{SubjectDigest: input.SubjectDigest, EvidenceDigest: input.EvidenceDigest, PolicyDigest: input.PolicyDigest, PolicyDecision: decision, Verification: "unverified", EvidenceReferences: refs}}, nil
}

func policyHash(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func evaluationInterrupted(ctx context.Context) (contracts.Result, error) {
	code := "cancelled"
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		code = "deadline_exceeded"
	}
	return contracts.Result{}, &contracts.Error{Code: code, Field: "$", Message: "Release evaluation interrupted."}
}
