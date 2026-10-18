package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"attestara/internal/contracts"
	"attestara/internal/policy"
	"attestara/internal/storage"
)

func c14(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C14", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func c14Hash(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func TestC14FixtureProvenance(t *testing.T) {
	var provenance struct {
		SchemaVersion int               `json:"schema_version"`
		License       string            `json:"license"`
		Files         map[string]string `json:"files"`
	}
	if err := json.Unmarshal(c14(t, "provenance.json"), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.SchemaVersion != 1 || provenance.License != "Apache-2.0" || len(provenance.Files) < 10 {
		t.Fatal("fixture provenance missing")
	}
	entries, err := os.ReadDir("fixtures/attestara/C14")
	if err != nil {
		t.Fatal(err)
	}
	if len(provenance.Files) != len(entries)-1 {
		t.Fatal("unlisted fixture")
	}
	for _, entry := range entries {
		if entry.Name() == "provenance.json" {
			continue
		}
		if entry.IsDir() || provenance.Files[entry.Name()] != c14Hash(c14(t, entry.Name())) {
			t.Fatal("fixture checksum mismatch", entry.Name())
		}
	}
}
func TestC14DecisionsAndExactBindings(t *testing.T) {
	var fixtures struct {
		SchemaVersion int `json:"schema_version"`
		Cases         []struct {
			ID              string `json:"id"`
			InputFixture    string `json:"input_fixture"`
			EvidenceFixture string `json:"evidence_fixture"`
			Expected        string `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c14(t, "positive.json"), &fixtures); err != nil {
		t.Fatal(err)
	}
	if fixtures.SchemaVersion != 1 || len(fixtures.Cases) != 3 {
		t.Fatal("positive cases missing")
	}
	var expected struct {
		RuleDigest string                      `json:"rule_digest"`
		Reference  contracts.EvidenceReference `json:"reference"`
	}
	if err := json.Unmarshal(c14(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixtures.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			result, err := policy.Evaluate(context.Background(), c14(t, tc.InputFixture), c14(t, "rule.json"), c14(t, tc.EvidenceFixture))
			if err != nil {
				t.Fatal(err)
			}
			o := result.Result
			if result.SchemaVersion != "2" || o.PolicyDecision != tc.Expected || o.PolicyDigest != expected.RuleDigest || o.Verification != "unverified" || !reflect.DeepEqual(o.EvidenceReferences, []contracts.EvidenceReference{{Path: tc.EvidenceFixture, Digest: c14Hash(c14(t, tc.EvidenceFixture))}}) || result.InputDigest != c14Hash(c14(t, tc.InputFixture)) {
				t.Fatalf("incorrect result: %+v", result)
			}
			b, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := contracts.DecodeResult(context.Background(), b)
			if err != nil || !reflect.DeepEqual(decoded, result) {
				t.Fatalf("round trip: %v", err)
			}
			again, err := policy.Evaluate(context.Background(), c14(t, tc.InputFixture), c14(t, "rule.json"), c14(t, tc.EvidenceFixture))
			if err != nil || !reflect.DeepEqual(result, again) {
				t.Fatal("non deterministic result", err)
			}
		})
	}
	r, err := policy.Evaluate(context.Background(), c14(t, "input.json"), c14(t, "rule.json"), nil)
	if err != nil || r.Result.PolicyDecision != "unknown" || r.Result.EvidenceReferences == nil || len(r.Result.EvidenceReferences) != 0 {
		t.Fatal("missing evidence passed", r, err)
	}
}
func TestC14RejectsInvalidRulesAndBytes(t *testing.T) {
	var negative struct {
		Cases []struct {
			Rule string `json:"rule"`
			Code string `json:"code"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c14(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) < 7 {
		t.Fatal("negative cases missing")
	}
	reject := func(input, rule, evidence []byte, ctx context.Context, code string) {
		t.Helper()
		r, err := policy.Evaluate(ctx, input, rule, evidence)
		var e *contracts.Error
		if !errors.As(err, &e) || e.Code != code || !reflect.DeepEqual(r, contracts.Result{}) || strings.Contains(e.Message, "sensitive-value") {
			t.Fatalf("expected %s, got %+v %v", code, r, err)
		}
	}
	for _, tc := range negative.Cases {
		reject(c14(t, "input.json"), []byte(tc.Rule), nil, context.Background(), tc.Code)
	}
	reject(c14(t, "input.json"), append(c14(t, "rule.json"), ' '), nil, context.Background(), "invalid_input")
	reject(c14(t, "input.json"), c14(t, "rule.json"), []byte("modified"), context.Background(), "invalid_input")
	reject(c14(t, "input.json"), bytes.Repeat([]byte(" "), 4097), nil, context.Background(), "limit_exceeded")
	reject(c14(t, "input.json"), []byte{0xff}, nil, context.Background(), "invalid_json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reject(c14(t, "input.json"), c14(t, "rule.json"), nil, ctx, "cancelled")
	marker := filepath.Join(t.TempDir(), "executed")
	reject(c14(t, "input.json"), []byte("touch "+marker), nil, context.Background(), "invalid_json")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("source executed", err)
	}
}
func TestC14StoredV2AndV1Compatibility(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "store")
	a, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := storageFixture(t)
	if err := a.Commit(ctx, old); err != nil {
		t.Fatal(err)
	}
	r, err := policy.Evaluate(ctx, c14(t, "input.json"), c14(t, "rule.json"), c14(t, "evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	record := storage.Record{OperationID: "c14", InputJSON: c14(t, "input.json"), ResultJSON: b, Files: map[string][]byte{"subject.txt": c14(t, "subject.txt"), "evidence.json": c14(t, "evidence.json"), "rule.json": c14(t, "rule.json")}}
	if err := a.Commit(ctx, record); err != nil {
		t.Fatal(err)
	}
	a.Close()
	a, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, want := range []storage.Record{old, record} {
		got, err := a.Load(ctx, want.OperationID)
		if err != nil || !bytes.Equal(got.ResultJSON, want.ResultJSON) {
			t.Fatal("restart lost result", err)
		}
	}
	for _, suffix := range []string{".fail", ".unknown"} {
		inputJSON := c14(t, "input"+suffix+".json")
		evidenceName := "evidence" + suffix + ".json"
		evaluated, err := policy.Evaluate(ctx, inputJSON, c14(t, "rule.json"), c14(t, evidenceName))
		if err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(evaluated)
		if err != nil {
			t.Fatal(err)
		}
		next := storage.Record{OperationID: "c14" + suffix, InputJSON: inputJSON, ResultJSON: wire, Files: map[string][]byte{"subject.txt": c14(t, "subject.txt"), "rule.json": c14(t, "rule.json"), evidenceName: c14(t, evidenceName)}}
		if err := a.Commit(ctx, next); err != nil {
			t.Fatal(err)
		}
		a.Close()
		a, err = storage.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		loaded, err := a.Load(ctx, next.OperationID)
		if err != nil || !bytes.Equal(wire, loaded.ResultJSON) {
			t.Fatal("decision lost on reopen", err)
		}
		again, err := policy.Evaluate(ctx, loaded.InputJSON, loaded.Files["rule.json"], loaded.Files[evidenceName])
		if err != nil || !reflect.DeepEqual(again, evaluated) {
			t.Fatal("stored evidence replay differs", err)
		}
	}
	before, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Commit(ctx, record); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "manifest"))
	if !bytes.Equal(before, after) {
		t.Fatal("lost ACK replay duplicated effect")
	}
	record.ResultJSON = append(record.ResultJSON, ' ')
	if err := a.Commit(ctx, record); !errors.Is(err, storage.ErrConflict) {
		t.Fatal("changed retry accepted", err)
	}
	// A syntactically valid reference must still belong to the input manifest.
	r.Result.EvidenceReferences[0].Path = "not-in-manifest.json"
	record.ResultJSON, _ = json.Marshal(r)
	record.OperationID = "forged"
	if err := a.Commit(ctx, record); err == nil {
		t.Fatal("unbound reference accepted")
	}
	missing, err := policy.Evaluate(ctx, c14(t, "input.json"), c14(t, "rule.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	record.ResultJSON, _ = json.Marshal(missing)
	delete(record.Files, "evidence.json")
	if err := a.Commit(ctx, record); err == nil {
		t.Fatal("incomplete evidence bundle persisted")
	}
}

func TestC14EvidenceFactsStrictnessAndRuleByteIdentity(t *testing.T) {
	// Rebind manifest bytes so these cases reach fact parsing rather than the
	// earlier byte-integrity guard. No caller-provided boolean can override facts.
	rebind := func(rule, evidence []byte) []byte {
		t.Helper()
		var input contracts.Input
		if err := json.Unmarshal(c14(t, "input.json"), &input); err != nil {
			t.Fatal(err)
		}
		input.PolicyDigest = c14Hash(rule)
		input.EvidenceDigest = c14Hash(evidence)
		for i := range input.Files {
			f := &input.Files[i]
			switch f.Path {
			case "rule.json":
				f.Digest = input.PolicyDigest
				n := int64(len(rule))
				f.SizeBytes = &n
			case "evidence.json":
				f.Digest = input.EvidenceDigest
				n := int64(len(evidence))
				f.SizeBytes = &n
			}
		}
		b, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, tc := range []struct{ bytes, code string }{
		{"{", "invalid_json"}, {`{"schema_version":"1","evidence_present":true,"evidence_present":false}`, "invalid_json"},
		{`{"schema_version":"1","evidence_present":"sensitive-value"}`, "invalid_input"},
		{`{"schema_version":"1","evidence_present":true,"command":"sensitive-value"}`, "invalid_input"},
		{`{"schema_version":"2","evidence_present":true}`, "unsupported_version"}, {`null`, "invalid_input"},
	} {
		evidence := []byte(tc.bytes)
		r, err := policy.Evaluate(context.Background(), rebind(c14(t, "rule.json"), evidence), c14(t, "rule.json"), evidence)
		var typed *contracts.Error
		if !errors.As(err, &typed) || typed.Code != tc.code || !reflect.DeepEqual(r, contracts.Result{}) || strings.Contains(typed.Message, "sensitive-value") {
			t.Fatal(tc, err)
		}
	}
	rule := append(c14(t, "rule.json"), ' ')
	evidence := c14(t, "evidence.json")
	r, err := policy.Evaluate(context.Background(), rebind(rule, evidence), rule, evidence)
	if err != nil || r.Result.PolicyDecision != "pass" || r.Result.PolicyDigest != c14Hash(rule) {
		t.Fatal("rule bytes canonicalized", err)
	}
}
