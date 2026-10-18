package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	jsonv2 "encoding/json/v2"

	"attestara/internal/contracts"
	"attestara/internal/decision"
	"attestara/internal/policy"
	"attestara/internal/report"
)

func c19Read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C19", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type c19Case struct {
	ID                    string `json:"id"`
	InputFixture          string `json:"input_fixture"`
	EvidenceFixture       string `json:"evidence_fixture"`
	PolicyFixture         string `json:"policy_fixture"`
	DuplicateEvidencePath string `json:"duplicate_evidence_path"`
	Freshness             string `json:"freshness"`
}

func c19Positive(t *testing.T) c19Case {
	t.Helper()
	var fixture struct {
		SchemaVersion int       `json:"schema_version"`
		License       string    `json:"license"`
		Cases         []c19Case `json:"cases"`
	}
	if err := json.Unmarshal(c19Read(t, "positive.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.License != "Apache-2.0" || len(fixture.Cases) != 1 || fixture.Cases[0].ID != "stable-evidence-order" {
		t.Fatal("missing versioned C19 positive fixture")
	}
	return fixture.Cases[0]
}

func c19Build(t *testing.T, kind string, duplicate, reverse bool, freshness decision.Freshness) (contracts.Input, contracts.Result) {
	t.Helper()
	inputName, evidenceName := "input.json", "evidence.json"
	switch kind {
	case "pass":
	case "fail", "unknown":
		inputName, evidenceName = "input."+kind+".json", "evidence."+kind+".json"
	default:
		t.Fatalf("unknown C19 result case %q", kind)
	}
	inputBytes := c14(t, inputName)
	input, err := contracts.DecodeInput(context.Background(), inputBytes)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		var reference *contracts.File
		for i := range input.Files {
			if input.Files[i].Digest == input.EvidenceDigest {
				reference = &input.Files[i]
				break
			}
		}
		if reference == nil {
			t.Fatal("C14 fixture has no evidence file")
		}
		size := *reference.SizeBytes
		input.Files = append(input.Files, contracts.File{Path: c19Positive(t).DuplicateEvidencePath, Digest: reference.Digest, SizeBytes: &size, MediaType: reference.MediaType})
	}
	if reverse {
		for left, right := 0, len(input.Files)-1; left < right; left, right = left+1, right-1 {
			input.Files[left], input.Files[right] = input.Files[right], input.Files[left]
		}
	}
	inputBytes, err = jsonv2.Marshal(input, jsonv2.FormatNilSliceAsNull(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := contracts.DecodeInput(context.Background(), inputBytes); err != nil {
		t.Fatal("generated C19 manifest is invalid", err)
	}
	result, err := policy.Evaluate(context.Background(), inputBytes, c14(t, "rule.json"), c14(t, evidenceName))
	if err != nil {
		t.Fatal(err)
	}
	result, err = decision.Explain(context.Background(), result, freshness)
	if err != nil {
		t.Fatal(err)
	}
	return input, result
}

func TestC19PassportDeterminism(t *testing.T) {
	positive := c19Positive(t)
	if positive.InputFixture != "C14/input.json" || positive.EvidenceFixture != "C14/evidence.json" || positive.PolicyFixture != "C14/rule.json" || positive.Freshness != "fresh" {
		t.Fatal("C19 fixture does not identify its real C14 inputs")
	}
	if !bytes.Equal(c19Read(t, "evidence-copy.json"), c14(t, "evidence.json")) {
		t.Fatal("duplicate C19 evidence bytes differ from their C14 source")
	}
	input, result := c19Build(t, "pass", true, false, decision.Fresh)
	got, err := report.GeneratePassport(context.Background(), input, result)
	if err != nil {
		t.Fatal(err)
	}
	var want report.Passport
	if err := json.Unmarshal(c19Read(t, "expected.json"), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		actual, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("passport mismatch\n%s", actual)
	}

	reversedInput, reversedResult := c19Build(t, "pass", true, true, decision.Fresh)
	reversedRefs := append([]contracts.EvidenceReference(nil), reversedResult.Result.EvidenceReferences...)
	reversed, err := report.GeneratePassport(context.Background(), reversedInput, reversedResult)
	if err != nil || !reflect.DeepEqual(reversed, got) {
		t.Fatalf("evidence order changed normalized passport: %+v / %v", reversed, err)
	}
	if !reflect.DeepEqual(reversedResult.Result.EvidenceReferences, reversedRefs) {
		t.Fatal("passport generation mutated the C18 result")
	}

	input.SourceRevision += "-changed"
	changed, err := report.GeneratePassport(context.Background(), input, result)
	if err != nil || changed.ReportDigest == got.ReportDigest {
		t.Fatal("meaningful passport content did not change the digest", err)
	}
}

func TestC19PolicyOutcomesAndTrustRemainIndependent(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		decision decision.Freshness
		want     string
		warnings []string
	}{
		{kind: "pass", decision: decision.Fresh, want: "pass", warnings: []string{}},
		{kind: "fail", decision: decision.Fresh, want: "fail", warnings: []string{decision.PolicyFailed}},
		{kind: "unknown", decision: decision.Fresh, want: "unknown", warnings: []string{}},
		{kind: "pass", decision: decision.Stale, want: "unknown", warnings: []string{decision.EvidenceStale}},
	} {
		t.Run(tc.kind+"/"+string(tc.decision), func(t *testing.T) {
			input, result := c19Build(t, tc.kind, false, false, tc.decision)
			got, err := report.GeneratePassport(context.Background(), input, result)
			if err != nil || got.PolicyDecision != tc.want || got.Verification != "unverified" || got.Status != result.Status || !reflect.DeepEqual(got.Warnings, tc.warnings) {
				t.Fatalf("policy/trust projection changed: %+v / %v", got, err)
			}
		})
	}
	input, result := c19Build(t, "pass", false, false, decision.Fresh)
	result.Status = "partial"
	result.Result.Verification = "rejected"
	got, err := report.GeneratePassport(context.Background(), input, result)
	if err != nil || got.Status != "partial" || got.Verification != "rejected" {
		t.Fatalf("rejection was promoted or lost: %+v / %v", got, err)
	}
}

func TestC19RejectsInvalidInputsAndBindings(t *testing.T) {
	var negative struct {
		SchemaVersion int `json:"schema_version"`
		License       string
		Cases         []struct {
			ID, Code, Field string
		}
	}
	if err := json.Unmarshal(c19Read(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if negative.SchemaVersion != 1 || negative.License != "Apache-2.0" || len(negative.Cases) != 7 {
		t.Fatal("missing versioned C19 negative cases")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			input, result := c19Build(t, "pass", true, false, decision.Fresh)
			ctx := context.Background()
			switch tc.ID {
			case "invalid-input-version":
				input.SchemaVersion = "3"
			case "invalid-result-version":
				result.SchemaVersion = "3"
			case "subject-mismatch":
				result.Result.SubjectDigest = "sha256:" + strings.Repeat("0", 64)
			case "evidence-mismatch":
				result.Result.EvidenceDigest = "sha256:" + strings.Repeat("0", 64)
			case "policy-mismatch":
				result.Result.PolicyDigest = "sha256:" + strings.Repeat("0", 64)
			case "unbound-reference":
				result.Result.EvidenceReferences[0].Path = "not-in-manifest.json"
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			default:
				t.Fatalf("unknown C19 negative case %q", tc.ID)
			}
			got, err := report.GeneratePassport(ctx, input, result)
			var typed *contracts.Error
			if !reflect.DeepEqual(got, report.Passport{}) || !errors.As(err, &typed) || typed.Code != tc.Code || typed.Field != tc.Field {
				t.Fatalf("unsafe or incorrect rejection: %+v / %v; want %s %s", got, err, tc.Code, tc.Field)
			}
		})
	}
}

func TestC19FailedEvaluationAndSourceSentinelStayInert(t *testing.T) {
	input, result := c19Build(t, "pass", true, false, decision.Fresh)
	result.Status = "failed"
	result.Result = nil
	result.Error = &contracts.Error{Code: "dependency_unavailable", Field: "synthetic-secret", Message: "synthetic-secret"}
	_, err := report.GeneratePassport(context.Background(), input, result)
	var typed *contracts.Error
	if !errors.As(err, &typed) || typed.Code != "dependency_unavailable" || strings.Contains(err.Error(), "synthetic-secret") {
		t.Fatal("failed evaluation became a passport or leaked caller text", err)
	}

	marker := filepath.Join(t.TempDir(), "executed")
	input, result = c19Build(t, "pass", true, false, decision.Fresh)
	result.Warnings = []string{"touch " + marker}
	passport, err := report.GeneratePassport(context.Background(), input, result)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(passport.Warnings, []string{"[REDACTED]"}) {
		t.Fatal("free-text warning reached the public passport", passport.Warnings)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("source-like warning executed", err)
	}
}

func TestC19FixtureProvenance(t *testing.T) {
	var provenance struct {
		SchemaVersion int               `json:"schema_version"`
		License       string            `json:"license"`
		Files         map[string]string `json:"files"`
	}
	if err := json.Unmarshal(c19Read(t, "provenance.json"), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.SchemaVersion != 1 || provenance.License != "Apache-2.0" {
		t.Fatal("invalid C19 provenance")
	}
	entries, err := os.ReadDir("fixtures/attestara/C19")
	if err != nil {
		t.Fatal(err)
	}
	if len(provenance.Files) != len(entries)-1+8 {
		t.Fatal("C19 provenance does not cover every fixture and reused input")
	}
	for _, entry := range entries {
		if entry.Name() == "provenance.json" {
			continue
		}
		b := c19Read(t, entry.Name())
		if provenance.Files["C19/"+entry.Name()] != fmt.Sprintf("sha256:%x", sha256.Sum256(b)) {
			t.Fatal("C19 fixture hash mismatch", entry.Name())
		}
	}
	for _, name := range []string{"input.json", "input.fail.json", "input.unknown.json", "evidence.json", "evidence.fail.json", "evidence.unknown.json", "rule.json", "subject.txt"} {
		b := c14(t, name)
		if provenance.Files["C14/"+name] != fmt.Sprintf("sha256:%x", sha256.Sum256(b)) {
			t.Fatal("referenced C14 fixture hash mismatch", name)
		}
	}
}
