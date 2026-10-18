package tests

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"attestara/internal/adapters"
	"attestara/internal/contracts"
	"attestara/internal/storage"
)

func c15(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C15", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func opaExecutable(t *testing.T) string {
	t.Helper()
	path := os.Getenv("ATTESTARA_TEST_OPA")
	if path == "" {
		path, _ = exec.LookPath("opa")
	}
	if path == "" {
		t.Skip("optional OPA absent; real C15 provider acceptance is NOT verified")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func opaRebind(t *testing.T, policy, evidence []byte) []byte {
	t.Helper()
	var input contracts.Input
	if err := json.Unmarshal(c15(t, "input.pass.json"), &input); err != nil {
		t.Fatal(err)
	}
	input.PolicyDigest = c14Hash(policy)
	input.EvidenceDigest = c14Hash(evidence)
	for i, data := range [][]byte{nil, evidence, policy} {
		if i > 0 {
			n := int64(len(data))
			input.Files[i].Digest = c14Hash(data)
			input.Files[i].SizeBytes = &n
		}
	}
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func opaReject(t *testing.T, result contracts.Result, err error, code string) {
	t.Helper()
	var typed *contracts.Error
	if !reflect.DeepEqual(result, contracts.Result{}) || !errors.As(err, &typed) || typed.Code != code || typed.Message == "" {
		t.Fatalf("got %+v / %v; want empty result and %s", result, err, code)
	}
	if strings.Contains(err.Error(), "synthetic-secret") {
		t.Fatal("provider input leaked")
	}
}
func TestC15RealOPAAndBindings(t *testing.T) {
	executable := opaExecutable(t)
	t.Setenv("OPA_EVAL_FORMAT", "pretty")
	t.Setenv("ATTESTARA_SYNTHETIC_SECRET", "synthetic-secret")
	if !bytes.Equal(c15(t, "policy.rego"), readC04Policy(t)) {
		t.Fatal("C04 policy changed")
	}
	var fixtures struct {
		Cases []struct {
			ID       string `json:"id"`
			Input    string `json:"input_fixture"`
			Evidence string `json:"evidence_fixture"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c15(t, "positive.json"), &fixtures); err != nil || len(fixtures.Cases) != 3 {
		t.Fatal("fixture cases missing", err)
	}
	for _, tc := range fixtures.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			input, evidence, policy := c15(t, tc.Input), c15(t, tc.Evidence), c15(t, "policy.rego")
			before := bytes.Clone(input)
			got, err := adapters.EvaluateOPA(context.Background(), executable, input, policy, evidence)
			if err != nil {
				t.Fatal(err)
			}
			wantRef := []contracts.EvidenceReference{{Path: "evidence.pass.json", Digest: c14Hash(evidence)}}
			wantRef[0].Path = tc.Evidence
			if got.SchemaVersion != "2" || got.Status != "unverified" || got.Result == nil || got.Result.PolicyDecision != tc.Expected || got.Result.Verification != "unverified" || got.Result.PolicyDigest != c14Hash(policy) || got.Result.EvidenceDigest != c14Hash(evidence) || got.InputDigest != c14Hash(input) || !reflect.DeepEqual(got.Result.EvidenceReferences, wantRef) || !bytes.Equal(input, before) {
				t.Fatalf("wrong binding: %+v", got)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := contracts.DecodeResult(context.Background(), encoded)
			if err != nil || !reflect.DeepEqual(decoded, got) {
				t.Fatal("invalid v2 result", err)
			}
			again, err := adapters.EvaluateOPA(context.Background(), executable, input, policy, evidence)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("fresh OPA process changed result", err)
			}
		})
	}
}
func readC04Policy(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("fixtures/attestara/C04/policy.rego")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestC15RejectsInvalidInputs(t *testing.T) {
	policy, evidence := c15(t, "policy.rego"), c15(t, "evidence.pass.json")
	var fixtures struct {
		Cases []struct {
			ID       string `json:"id"`
			Evidence string `json:"evidence"`
			Code     string `json:"code"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c15(t, "negative.json"), &fixtures); err != nil || len(fixtures.Cases) != 5 {
		t.Fatal("negative cases missing", err)
	}
	for _, tc := range fixtures.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			b := []byte(tc.Evidence)
			r, e := adapters.EvaluateOPA(context.Background(), "/absent/opa", opaRebind(t, policy, b), policy, b)
			opaReject(t, r, e, tc.Code)
		})
	}
	for _, tc := range []struct {
		input, policy, evidence []byte
		code                    string
	}{
		{[]byte("{"), policy, evidence, "invalid_json"},
		{c15(t, "input.pass.json"), policy, append(bytes.Clone(evidence), ' '), "invalid_input"},
		{opaRebind(t, []byte("#!/bin/sh\nsynthetic-secret"), evidence), []byte("#!/bin/sh\nsynthetic-secret"), evidence, "invalid_input"},
		{c15(t, "input.pass.json"), policy, nil, "input_unavailable"},
		{c15(t, "input.pass.json"), bytes.Repeat([]byte("x"), 4097), evidence, "limit_exceeded"},
		{c15(t, "input.pass.json"), policy, make([]byte, contracts.MaxBundleBytes+1), "limit_exceeded"},
	} {
		r, e := adapters.EvaluateOPA(context.Background(), "/absent/opa", tc.input, tc.policy, tc.evidence)
		opaReject(t, r, e, tc.code)
	}
	var manifest contracts.Input
	_ = json.Unmarshal(c15(t, "input.pass.json"), &manifest)
	*manifest.Files[1].SizeBytes++
	bad, _ := json.Marshal(manifest)
	r, e := adapters.EvaluateOPA(context.Background(), "/absent/opa", bad, policy, evidence)
	opaReject(t, r, e, "invalid_input")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = adapters.EvaluateOPA(ctx, "/absent/opa", c15(t, "input.pass.json"), policy, evidence)
	opaReject(t, r, e, "cancelled")
}
func fakeOPA(t *testing.T, version, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opa")
	data := "#!/bin/sh\nif [ \"$1\" = version ]; then printf 'Version: " + version + "\\n'; exit; fi\n" + body + "\n"
	if err := os.WriteFile(path, []byte(data), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestC15ProviderFailuresAndTimeout(t *testing.T) {
	for _, tc := range []struct{ name, path, code string }{
		{"missing", "/nonexistent/opa", "dependency_unavailable"},
		{"relative", "opa", "dependency_unavailable"},
		{"version", fakeOPA(t, "0.0.0", "exit 0"), "dependency_unavailable"},
		{"failed", fakeOPA(t, "1.21.2", "echo synthetic-secret >&2; exit 1"), "invalid_input"},
		{"malformed", fakeOPA(t, "1.21.2", "printf '{'"), "invalid_json"},
		{"undefined", fakeOPA(t, "1.21.2", "printf '{}'"), "invalid_input"},
		{"wrong-type", fakeOPA(t, "1.21.2", `printf '%s' '{"result":[{"expressions":[{"value":"synthetic-secret"}]}]}'`), "invalid_input"},
		{"duplicate", fakeOPA(t, "1.21.2", `printf '%s' '{"result":[],"result":[]}'`), "invalid_json"},
		{"stderr-limit", fakeOPA(t, "1.21.2", "while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' >&2; done"), "limit_exceeded"},
		{"oversized", fakeOPA(t, "1.21.2", "while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'; done"), "limit_exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := adapters.EvaluateOPA(context.Background(), tc.path, c15(t, "input.pass.json"), c15(t, "policy.rego"), c15(t, "evidence.pass.json"))
			opaReject(t, r, e, tc.code)
		})
	}
	path := fakeOPA(t, "1.21.2", "while :; do :; done")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	r, e := adapters.EvaluateOPA(ctx, path, c15(t, "input.pass.json"), c15(t, "policy.rego"), c15(t, "evidence.pass.json"))
	opaReject(t, r, e, "deadline_exceeded")
	if time.Since(started) > 2*time.Second {
		t.Fatal("provider did not terminate promptly")
	}
}

func TestC15FixtureProvenance(t *testing.T) {
	var p struct {
		SchemaVersion int               `json:"schema_version"`
		License       string            `json:"license"`
		Files         map[string]string `json:"files"`
	}
	if err := json.Unmarshal(c15(t, "provenance.json"), &p); err != nil || p.SchemaVersion != 1 || p.License != "Apache-2.0" {
		t.Fatal("invalid provenance", err)
	}
	entries, err := os.ReadDir("fixtures/attestara/C15")
	if err != nil || len(p.Files) != len(entries)-1 {
		t.Fatal("missing provenance", err)
	}
	for _, entry := range entries {
		if entry.Name() != "provenance.json" && (entry.IsDir() || p.Files[entry.Name()] != c14Hash(c15(t, entry.Name()))) {
			t.Fatal("invalid fixture hash", entry.Name())
		}
	}
	var expected struct {
		Digest   string `json:"policy_digest"`
		Query    string `json:"query"`
		Timeout  string `json:"timeout_code"`
		Provider string `json:"provider_code"`
	}
	if json.Unmarshal(c15(t, "expected.json"), &expected) != nil || expected.Digest != c14Hash(readC04Policy(t)) || expected.Query != "data.attestara.release.allow" || expected.Timeout != "deadline_exceeded" || expected.Provider != "dependency_unavailable" {
		t.Fatal("wrong expected contract")
	}
}
func TestC15ReopenAndFreshProcess(t *testing.T) {
	executable := opaExecutable(t)
	ctx := context.Background()
	input, policy, evidence := c15(t, "input.pass.json"), c15(t, "policy.rego"), c15(t, "evidence.pass.json")
	result, err := adapters.EvaluateOPA(ctx, executable, input, policy, evidence)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ATTESTARA_C15_REPEAT") == "1" {
		t.Log(c14Hash(wire))
		return
	}
	dir := filepath.Join(t.TempDir(), "store")
	store, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := storage.Record{OperationID: "c15", InputJSON: input, ResultJSON: wire, Files: map[string][]byte{"subject.txt": c15(t, "subject.txt"), "policy.rego": policy, "evidence.pass.json": evidence}}
	if err = store.Commit(ctx, record); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	loaded, err := store.Load(ctx, "c15")
	if err != nil || !bytes.Equal(loaded.ResultJSON, wire) {
		t.Fatal("OPA result lost on reopen", err)
	}
	again, err := adapters.EvaluateOPA(ctx, executable, loaded.InputJSON, loaded.Files["policy.rego"], loaded.Files["evidence.pass.json"])
	if err != nil || !reflect.DeepEqual(again, result) {
		t.Fatal("reopened evaluation changed", err)
	}
	childCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(childCtx, os.Args[0], "-test.run=^TestC15ReopenAndFreshProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "ATTESTARA_C15_REPEAT=1", "ATTESTARA_TEST_OPA="+executable)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), c14Hash(wire)) {
		t.Fatal("fresh engine process changed result", err, string(out))
	}
}

func TestC15NoSourceExecutionAndEnvironment(t *testing.T) {
	t.Setenv("ATTESTARA_SYNTHETIC_SECRET", "synthetic-secret")
	t.Setenv("OPA_EVAL_FORMAT", "pretty")
	provider := fakeOPA(t, "1.21.2", `[ -z "$ATTESTARA_SYNTHETIC_SECRET" ] && [ -z "$OPA_EVAL_FORMAT" ] || exit 1
printf '%s' '{"result":[{"expressions":[{"value":true}]}]}'`)
	r, e := adapters.EvaluateOPA(context.Background(), provider, c15(t, "input.pass.json"), c15(t, "policy.rego"), c15(t, "evidence.pass.json"))
	if e != nil || r.Result.PolicyDecision != "pass" {
		t.Fatal("host environment entered provider", e)
	}
	sentinel := filepath.Join(t.TempDir(), "executed")
	source := []byte("#!/bin/sh\n: > " + sentinel + "\n")
	r, e = adapters.EvaluateOPA(context.Background(), provider, opaRebind(t, source, c15(t, "evidence.pass.json")), source, c15(t, "evidence.pass.json"))
	opaReject(t, r, e, "invalid_input")
	if _, e = os.Stat(sentinel); !os.IsNotExist(e) {
		t.Fatal("source executed", e)
	}
}
