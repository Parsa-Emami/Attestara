// Package decision explains existing policy results using caller-classified
// evidence freshness. It does not determine expiry or establish authenticity.
package decision

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"slices"

	"attestara/internal/contracts"
)

type Freshness string

const (
	Fresh Freshness = "fresh"
	Stale Freshness = "stale"

	EvidenceMissing = "evidence_missing"
	EvidenceStale   = "evidence_stale"
	PolicyFailed    = "policy_failed"
)

// Explain returns an independently owned v2 result with stable reason codes in
// warnings. The caller supplies freshness independently of scanner findings and
// reported database validity. Failed evaluation envelopes retain their error;
// raw parser/provider Go errors must be handled before calling this function.
// Unknown with available references retains unknown without inferring its cause.
func Explain(parent context.Context, result contracts.Result, freshness Freshness) (contracts.Result, error) {
	ctx, cancel := context.WithTimeout(parent, contracts.ValidationDeadline)
	defer cancel()
	fail := func(code, field string) (contracts.Result, error) {
		return contracts.Result{}, &contracts.Error{Code: code, Field: field, Message: "Decision input is invalid or unavailable."}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fail("deadline_exceeded", "$")
	}
	if ctx.Err() != nil {
		return fail("cancelled", "$")
	}
	if freshness != Fresh && freshness != Stale {
		return fail("invalid_input", "freshness")
	}
	// Reuse the result validator and copy nested slices/pointers together, so
	// explanation cannot mutate the evaluator's raw result or stored snapshot.
	data, err := json.Marshal(result, json.FormatNilSliceAsNull(true))
	if err != nil {
		return fail("invalid_input", "result")
	}
	out, err := contracts.DecodeResult(ctx, data)
	if err != nil {
		return contracts.Result{}, err
	}
	if out.SchemaVersion != "2" {
		return fail("unsupported_version", "schema_version")
	}
	if out.Status == "failed" {
		return out, nil
	}
	add := func(reason string) {
		if !slices.Contains(out.Warnings, reason) {
			out.Warnings = append(out.Warnings, reason)
		}
	}
	if len(out.Result.EvidenceReferences) == 0 {
		out.Result.PolicyDecision = "unknown"
		add(EvidenceMissing)
	}
	if freshness == Stale {
		out.Result.PolicyDecision = "unknown"
		add(EvidenceStale)
	} else if out.Result.PolicyDecision == "fail" {
		add(PolicyFailed)
	}
	// A complete result requires a known decision under the existing v2 contract.
	if out.Result.PolicyDecision == "unknown" && out.Status == "complete" {
		out.Status = "partial"
	}
	// Added reasons must also fit the existing serialized-result boundary.
	data, err = json.Marshal(out, json.FormatNilSliceAsNull(true))
	if err != nil {
		return fail("invalid_input", "result")
	}
	return contracts.DecodeResult(ctx, data)
}
