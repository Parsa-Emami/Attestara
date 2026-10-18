package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"attestara/internal/adapters"
	"attestara/internal/contracts"
	"attestara/internal/decision"
	"attestara/internal/policy"
)

func c18Read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C18", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func c18Policy(t *testing.T, kind string) contracts.Result {
	t.Helper()
	input, evidence := "input.json", "evidence.json"
	switch kind {
	case "fail", "unknown":
		input, evidence = "input."+kind+".json", "evidence."+kind+".json"
	case "pass", "missing", "complete":
	default:
		t.Fatal("unsupported policy fixture", kind)
	}
	b := c14(t, evidence)
	if kind == "missing" {
		b = nil
	}
	r, err := policy.Evaluate(context.Background(), c14(t, input), c14(t, "rule.json"), b)
	if err != nil {
		t.Fatal(err)
	}
	if kind == "complete" {
		r.Status = "complete"
	}
	return r
}

func TestC18DecisionReasons(t *testing.T) {
	var positive struct {
		SchemaVersion int `json:"schema_version"`
		Cases         []struct{ ID, Policy, Freshness string }
	}
	var expected struct {
		SchemaVersion int `json:"schema_version"`
		Cases         map[string]struct {
			Decision, Status string
			Warnings         []string
		}
	}
	if err := json.Unmarshal(c18Read(t, "positive.json"), &positive); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(c18Read(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	if positive.SchemaVersion != 1 || expected.SchemaVersion != 1 || len(positive.Cases) != 8 || len(expected.Cases) != 8 {
		t.Fatal("missing versioned cases")
	}
	for _, tc := range positive.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			r := c18Policy(t, tc.Policy)
			before, _ := json.Marshal(r)
			got, err := decision.Explain(context.Background(), r, decision.Freshness(tc.Freshness))
			want, ok := expected.Cases[tc.ID]
			if !ok || err != nil || got.Result == nil || got.Result.PolicyDecision != want.Decision || got.Status != want.Status || !reflect.DeepEqual(got.Warnings, want.Warnings) {
				t.Fatalf("got %+v / %v; want %+v", got, err, want)
			}
			if got.Result.Verification != r.Result.Verification || got.InputDigest != r.InputDigest || got.Result.SubjectDigest != r.Result.SubjectDigest || got.Result.PolicyDigest != r.Result.PolicyDigest || got.Result.EvidenceDigest != r.Result.EvidenceDigest || !reflect.DeepEqual(got.Result.EvidenceReferences, r.Result.EvidenceReferences) {
				t.Fatal("changed bindings or trust")
			}
			wire, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := contracts.DecodeResult(context.Background(), wire)
			if err != nil || !reflect.DeepEqual(decoded, got) {
				t.Fatal("invalid v2 result", err)
			}
			after, _ := json.Marshal(r)
			if !bytes.Equal(before, after) {
				t.Fatal("mutated caller")
			}
			again, err := decision.Explain(context.Background(), got, decision.Freshness(tc.Freshness))
			if err != nil || !reflect.DeepEqual(again, got) {
				t.Fatal("repeated explanation changed result", err)
			}
		})
	}
}

func TestC18RejectsInvalidInputsAndCancellation(t *testing.T) {
	var negative struct {
		Cases []struct{ Mutation, Code string }
	}
	if err := json.Unmarshal(c18Read(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) != 14 {
		t.Fatal("missing negative cases")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.Mutation, func(t *testing.T) {
			r, freshness, ctx := c18Policy(t, "pass"), decision.Fresh, context.Background()
			switch tc.Mutation {
			case "empty-freshness":
				freshness = ""
			case "unsupported-freshness":
				freshness = "synthetic-secret"
			case "missing-result":
				r.Result = nil
			case "unsupported-result":
				r.SchemaVersion = "3"
			case "v1-result":
				r.SchemaVersion = "1"
				r.Result.PolicyDecision = "allow"
				r.Result.EvidenceReferences = nil
			case "bad-decision":
				r.Result.PolicyDecision = "synthetic-secret"
			case "missing-references":
				r.Result.EvidenceReferences = []contracts.EvidenceReference{}
			case "bad-digest":
				r.InputDigest = "synthetic-secret"
			case "failed-without-error":
				r.Status = "failed"
				r.Result = nil
			case "nil-warnings":
				r.Warnings = nil
			case "invalid-utf8":
				r.Warnings = []string{string([]byte{0xff})}
			case "output-limit":
				r.Warnings = []string{}
				wire, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				remaining := int(contracts.MaxBundleBytes) - len(wire) + 1
				// Keep each warning within the existing 4096-character limit.
				for remaining > 4099 {
					r.Warnings = append(r.Warnings, strings.Repeat("x", 4096))
					remaining -= 4099
				}
				if remaining <= 3 {
					t.Fatal("invalid boundary fixture")
				}
				r.Warnings = append(r.Warnings, strings.Repeat("x", remaining-3))
				freshness = decision.Stale
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "deadline":
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Unix(0, 0))
				defer cancel()
			default:
				t.Fatal("unknown mutation")
			}
			got, err := decision.Explain(ctx, r, freshness)
			var typed *contracts.Error
			if !reflect.DeepEqual(got, contracts.Result{}) || !errors.As(err, &typed) || typed.Code != tc.Code || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatalf("unsafe or incorrect failure: %+v / %v", got, err)
			}
		})
	}
}

func TestC18RealScannerAndPolicy(t *testing.T) {
	report, err := adapters.DecodeGrypeReport(c18Read(t, "grype.empty.json"))
	if err != nil || len(report.Findings) != 0 || report.DatabaseStatus != "reported_valid" || report.DatabaseAgeSeconds == nil {
		t.Fatal("invalid scanner fixture", report, err)
	}
	raw := c18Policy(t, "pass")
	got, err := decision.Explain(context.Background(), raw, decision.Stale)
	if err != nil || raw.Result.PolicyDecision != "pass" || got.Result.PolicyDecision != "unknown" || !reflect.DeepEqual(got.Warnings, []string{decision.EvidenceStale}) {
		t.Fatal("empty scanner findings bypassed stale evidence", got, err)
	}
}

func TestC18ErrorsStayFailed(t *testing.T) {
	_, parserErr := adapters.DecodeGrypeReport([]byte("{"))
	_, providerErr := adapters.EvaluateOPA(context.Background(), filepath.Join(t.TempDir(), "missing-opa"), c15(t, "input.pass.json"), c15(t, "policy.rego"), c15(t, "evidence.pass.json"))
	for i, err := range []error{parserErr, providerErr} {
		var typed *contracts.Error
		if !errors.As(err, &typed) || typed.Code != []string{"invalid_json", "dependency_unavailable"}[i] {
			t.Fatal("expected real typed parser/provider failure", err)
		}
		r := c18Policy(t, "pass")
		r.Status = "failed"
		r.Result = nil
		r.Error = typed
		for _, freshness := range []decision.Freshness{decision.Fresh, decision.Stale} {
			got, explainErr := decision.Explain(context.Background(), r, freshness)
			if explainErr != nil || !reflect.DeepEqual(got, r) {
				t.Fatal("converted failed evaluation", got, explainErr)
			}
			got.Error.Message = "changed"
			if r.Error.Message == "changed" {
				t.Fatal("failure aliases caller")
			}
		}
	}
}

func TestC18FreshProcessAndOwnership(t *testing.T) {
	r := c18Policy(t, "pass")
	got, err := decision.Explain(context.Background(), r, decision.Stale)
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := json.Marshal(got)
	digest := fmt.Sprintf("%x", sha256.Sum256(wire))
	if os.Getenv("ATTESTARA_C18_HELPER") == "1" {
		t.Log(digest)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestC18FreshProcessAndOwnership$", "-test.v")
	cmd.Env = append(os.Environ(), "ATTESTARA_C18_HELPER=1")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), digest) {
		t.Fatal("fresh process changed result", string(out), err)
	}
	marker := filepath.Join(t.TempDir(), "executed")
	r.Warnings = []string{"touch " + marker, decision.EvidenceStale}
	got, err = decision.Explain(context.Background(), r, decision.Stale)
	if err != nil || !reflect.DeepEqual(got.Warnings, r.Warnings) {
		t.Fatal("changed existing warnings or duplicated reason", err)
	}
	got.Warnings[0] = "changed"
	got.Result.EvidenceReferences[0].Path = "changed"
	got.Result.Verification = "rejected"
	if r.Warnings[0] == "changed" || r.Result.EvidenceReferences[0].Path == "changed" || r.Result.Verification != "unverified" {
		t.Fatal("output aliases caller")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("source-like warning executed", err)
	}
	r.Status = "complete"
	r.Result.Verification = "rejected"
	got, err = decision.Explain(context.Background(), r, decision.Stale)
	if err != nil || got.Status != "partial" || got.Result.Verification != "rejected" {
		t.Fatal("lost rejection during freshness assessment", got, err)
	}
}

func TestC18FixtureProvenance(t *testing.T) {
	var p struct {
		SchemaVersion int `json:"schema_version"`
		License       string
		Files         map[string]string
	}
	if err := json.Unmarshal(c18Read(t, "provenance.json"), &p); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("fixtures/attestara/C18")
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 1 || p.License != "Apache-2.0" || len(p.Files) != len(entries)-1 {
		t.Fatal("incomplete provenance")
	}
	for _, entry := range entries {
		if entry.Name() != "provenance.json" && (entry.IsDir() || p.Files[entry.Name()] != fmt.Sprintf("sha256:%x", sha256.Sum256(c18Read(t, entry.Name())))) {
			t.Fatal("fixture hash mismatch", entry.Name())
		}
	}
}
