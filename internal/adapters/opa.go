package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"attestara/internal/contracts"
)

// C02 pins OPA; this C15 slice executes only the existing C04 policy, not
// arbitrary uploaded Rego. The digest is checked against that fixture in tests.
const opaVersion = "1.21.2"
const opaPolicyDigest = "sha256:52f2452badde5b273ce44fb4663e08868116d8a1151544048021d8634b83483e"
const opaByteLimit = 4096

func opaHash(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func opaError(code, field string) error {
	return &contracts.Error{Code: code, Field: field, Message: "OPA evaluation input or provider is invalid or unavailable."}
}
func opaInterrupted(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return opaError("deadline_exceeded", "$")
	}
	if ctx.Err() != nil {
		return opaError("cancelled", "$")
	}
	return nil
}

// EvaluateOPA runs the pinned local CLI over the C04 allow policy and C14 facts.
// executable is an absolute path chosen by the application owner, never an
// uploaded path. Subject bytes are not opened or executed. Successful policy
// results remain authenticity-unverified. No result is returned on failure.
func EvaluateOPA(parent context.Context, executable string, inputJSON, policyBytes, evidenceBytes []byte) (contracts.Result, error) {
	ctx, cancel := context.WithTimeout(parent, contracts.ValidationDeadline)
	defer cancel()
	fail := func(code, field string) (contracts.Result, error) { return contracts.Result{}, opaError(code, field) }
	if err := opaInterrupted(ctx); err != nil {
		return contracts.Result{}, err
	}
	if len(policyBytes) > opaByteLimit || int64(len(evidenceBytes)) > contracts.MaxBundleBytes {
		return fail("limit_exceeded", "$")
	}
	input, err := contracts.DecodeInput(ctx, inputJSON)
	if err != nil {
		return contracts.Result{}, err
	}
	if opaHash(policyBytes) != opaPolicyDigest || input.PolicyDigest != opaPolicyDigest {
		return fail("invalid_input", "policy_digest")
	}
	if evidenceBytes == nil {
		return fail("input_unavailable", "evidence")
	}
	if opaHash(evidenceBytes) != input.EvidenceDigest {
		return fail("invalid_input", "evidence_digest")
	}
	if !jsontext.Value(evidenceBytes).IsValid() {
		return fail("invalid_json", "evidence")
	}
	var facts struct {
		SchemaVersion   string `json:"schema_version"`
		EvidencePresent *bool  `json:"evidence_present"`
	}
	if json.Unmarshal(evidenceBytes, &facts, json.RejectUnknownMembers(true)) != nil {
		return fail("invalid_input", "evidence")
	}
	if facts.SchemaVersion == "" {
		return fail("invalid_input", "evidence.schema_version")
	}
	if facts.SchemaVersion != "1" {
		return fail("unsupported_version", "evidence.schema_version")
	}
	refs := make([]contracts.EvidenceReference, 0)
	for _, file := range input.Files {
		if file.Digest == input.EvidenceDigest {
			if *file.SizeBytes != int64(len(evidenceBytes)) {
				return fail("invalid_input", "files.size_bytes")
			}
			refs = append(refs, contracts.EvidenceReference{Path: file.Path, Digest: file.Digest})
		}
		if file.Digest == input.PolicyDigest && *file.SizeBytes != int64(len(policyBytes)) {
			return fail("invalid_input", "files.size_bytes")
		}
	}
	if !filepath.IsAbs(executable) {
		return fail("dependency_unavailable", "opa")
	}
	version, err := runOPA(ctx, executable, "", nil, "version")
	if err != nil {
		var typed *contracts.Error
		if errors.As(err, &typed) && typed.Code == "invalid_input" {
			return fail("dependency_unavailable", "opa.version")
		}
		return contracts.Result{}, err
	}
	if !strings.Contains("\n"+string(version), "\nVersion: "+opaVersion+"\n") {
		return fail("dependency_unavailable", "opa.version")
	}
	dir, err := os.MkdirTemp("", "attestara-opa-")
	if err != nil {
		return fail("input_unavailable", "policy")
	}
	defer os.RemoveAll(dir)
	if os.WriteFile(filepath.Join(dir, "policy.rego"), policyBytes, 0600) != nil {
		return fail("input_unavailable", "policy")
	}
	// CLI options documented at https://www.openpolicyagent.org/docs/cli#eval.
	output, err := runOPA(ctx, executable, dir, evidenceBytes, "eval", "--format=json", "--stdin-input", "--timeout=10s", "--strict-builtin-errors", "--data=policy.rego", "data.attestara.release.allow")
	if err != nil {
		return contracts.Result{}, err
	}
	if !jsontext.Value(output).IsValid() {
		return fail("invalid_json", "opa.result")
	}
	var response struct {
		Errors jsontext.Value `json:"errors"`
		Result []struct {
			Expressions []struct {
				Value *bool `json:"value"`
			} `json:"expressions"`
		} `json:"result"`
	}
	if json.Unmarshal(output, &response) != nil || len(response.Errors) > 0 || len(response.Result) != 1 || len(response.Result[0].Expressions) != 1 || response.Result[0].Expressions[0].Value == nil {
		return fail("invalid_input", "opa.result")
	}
	if err := opaInterrupted(ctx); err != nil {
		return contracts.Result{}, err
	}
	decision := "fail"
	if *response.Result[0].Expressions[0].Value {
		decision = "pass"
	}
	return contracts.Result{SchemaVersion: "2", Status: "unverified", InputDigest: opaHash(inputJSON), EngineVersion: "0.0.1", Warnings: []string{}, Result: &contracts.Outcome{SubjectDigest: input.SubjectDigest, EvidenceDigest: input.EvidenceDigest, PolicyDigest: input.PolicyDigest, PolicyDecision: decision, Verification: "unverified", EvidenceReferences: refs}}, nil
}

type opaOutput struct {
	buffer   bytes.Buffer
	overflow bool
	cancel   context.CancelFunc
}

func (b *opaOutput) Write(p []byte) (int, error) {
	if len(p) > opaByteLimit-b.buffer.Len() {
		b.overflow = true
		b.cancel()
		return 0, errors.New("OPA output limit")
	}
	return b.buffer.Write(p)
}
func runOPA(parent context.Context, executable, dir string, input []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stdout, stderr := &opaOutput{cancel: cancel}, &opaOutput{cancel: cancel}
	cmd := exec.CommandContext(ctx, executable, args...)
	// OPA_* flags and opa.runtime must not inherit host configuration or secrets.
	cmd.Env = []string{}
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = 100 * time.Millisecond
	err := cmd.Run()
	if stdout.overflow || stderr.overflow {
		return nil, opaError("limit_exceeded", "opa.output")
	}
	if interrupted := opaInterrupted(parent); interrupted != nil {
		return nil, interrupted
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, opaError("invalid_input", "opa")
		}
		return nil, opaError("dependency_unavailable", "opa")
	}
	return stdout.buffer.Bytes(), nil
}
