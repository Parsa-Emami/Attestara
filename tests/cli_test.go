package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"attestara/internal/report"
)

type c20CommandResult struct {
	code   int
	stdout string
	stderr string
}

func c20Read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures/attestara/C20", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func buildC20CLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "attestara")
	command := exec.Command("go", "build", "-o", binary, "./cmd/attestara")
	command.Dir = ".."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func runC20CLI(t *testing.T, binary string, args ...string) c20CommandResult {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir = "."
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	return c20CommandResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func c20Passport(t *testing.T, output string) report.Passport {
	t.Helper()
	var passport report.Passport
	if err := json.Unmarshal([]byte(output), &passport); err != nil {
		t.Fatalf("decode CLI passport: %v\n%s", err, output)
	}
	return passport
}

func TestC20CLICommandsAndExitCodes(t *testing.T) {
	binary := buildC20CLI(t)
	input := "fixtures/attestara/C14/input.json"
	rule := "fixtures/attestara/C14/rule.json"
	evidence := "fixtures/attestara/C14/evidence.json"
	expectedBytes := c20Read(t, "expected.json")
	var expected report.Passport
	if err := json.Unmarshal(expectedBytes, &expected); err != nil {
		t.Fatal(err)
	}

	positive := runC20CLI(t, binary, "evaluate", "--input", input, "--policy", rule, "--evidence", evidence, "--freshness", "fresh")
	if positive.code != 0 || positive.stderr != "" {
		t.Fatalf("positive evaluate: %+v", positive)
	}
	if actual := c20Passport(t, positive.stdout); !reflect.DeepEqual(actual, expected) || actual.Verification != "unverified" {
		t.Fatalf("evaluate output mismatch or promoted trust: %+v", actual)
	}

	for _, tc := range []struct {
		name, input, evidence, freshness, wantDecision, wantWarning string
	}{
		{"policy-fail", "fixtures/attestara/C14/input.fail.json", "fixtures/attestara/C14/evidence.fail.json", "fresh", "fail", "policy_failed"},
		{"policy-unknown", "fixtures/attestara/C14/input.unknown.json", "fixtures/attestara/C14/evidence.unknown.json", "fresh", "unknown", ""},
		{"missing-evidence", input, "", "fresh", "unknown", "evidence_missing"},
		{"stale-evidence", input, evidence, "stale", "unknown", "evidence_stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"evaluate", "--input", tc.input, "--policy", rule, "--freshness", tc.freshness}
			if tc.evidence != "" {
				args = append(args, "--evidence", tc.evidence)
			}
			result := runC20CLI(t, binary, args...)
			if result.code != 1 || !strings.Contains(result.stderr, "policy decision is "+tc.wantDecision) {
				t.Fatalf("non-pass exit: %+v", result)
			}
			passport := c20Passport(t, result.stdout)
			if passport.PolicyDecision != tc.wantDecision || passport.Verification != "unverified" {
				t.Fatalf("non-pass result was lost or promoted: %+v", passport)
			}
			if tc.wantWarning != "" && !containsString(passport.Warnings, tc.wantWarning) {
				t.Fatalf("missing warning %q: %+v", tc.wantWarning, passport.Warnings)
			}
		})
	}

	for _, commandName := range []string{"verify", "export"} {
		t.Run(commandName, func(t *testing.T) {
			result := runC20CLI(t, binary, commandName, "--passport", "fixtures/attestara/C20/expected.json")
			if result.code != 0 || !reflectPassportEqual(c20Passport(t, result.stdout), expected) {
				t.Fatalf("%s output mismatch: %+v", commandName, result)
			}
			if commandName == "verify" && !strings.Contains(result.stderr, "signer and bundle trust were not assessed") {
				t.Fatalf("verify omitted its trust boundary: %q", result.stderr)
			}
		})
	}
	failResult := runC20CLI(t, binary, "evaluate", "--input", "fixtures/attestara/C14/input.fail.json", "--policy", rule, "--evidence", "fixtures/attestara/C14/evidence.fail.json", "--freshness", "fresh")
	if failResult.code != 1 {
		t.Fatalf("prepare non-pass passport: %+v", failResult)
	}
	failPath := filepath.Join(t.TempDir(), "non-pass.json")
	if err := os.WriteFile(failPath, []byte(failResult.stdout), 0600); err != nil {
		t.Fatal(err)
	}
	for _, commandName := range []string{"verify", "export"} {
		result := runC20CLI(t, binary, commandName, "--passport", failPath)
		if result.code != 1 || c20Passport(t, result.stdout).PolicyDecision != "fail" {
			t.Fatalf("%s lost the non-pass exit/result: %+v", commandName, result)
		}
	}

	t.Run("modified-passport", func(t *testing.T) {
		data := bytes.Replace(expectedBytes, []byte("synthetic-c14-1"), []byte("tampered-c14-1"), 1)
		path := filepath.Join(t.TempDir(), "modified.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		result := runC20CLI(t, binary, "verify", "--passport", path)
		if result.code != 2 || result.stdout != "" {
			t.Fatalf("modified passport accepted: %+v", result)
		}
	})

	t.Run("unsupported-passport-version", func(t *testing.T) {
		data := bytes.Replace(expectedBytes, []byte(`"schema_version": "1"`), []byte(`"schema_version": "2"`), 1)
		path := filepath.Join(t.TempDir(), "unsupported.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		result := runC20CLI(t, binary, "verify", "--passport", path)
		if result.code != 2 || result.stdout != "" {
			t.Fatalf("unsupported passport version accepted: %+v", result)
		}
	})

	t.Run("invalid-input", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.json")
		if err := os.WriteFile(path, []byte(`{"schema_version":"3"}`), 0600); err != nil {
			t.Fatal(err)
		}
		result := runC20CLI(t, binary, "evaluate", "--input", path, "--policy", rule, "--freshness", "fresh")
		if result.code != 2 || result.stdout != "" {
			t.Fatalf("invalid input did not fail as malformed: %+v", result)
		}
	})
	t.Run("malformed-input-json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "malformed.json")
		if err := os.WriteFile(path, []byte(`{"schema_version":`), 0600); err != nil {
			t.Fatal(err)
		}
		result := runC20CLI(t, binary, "evaluate", "--input", path, "--policy", rule, "--freshness", "fresh")
		if result.code != 2 || result.stdout != "" {
			t.Fatalf("malformed input was not rejected: %+v", result)
		}
	})
	t.Run("invalid-arguments", func(t *testing.T) {
		result := runC20CLI(t, binary, "evaluate", "--input")
		if result.code != 2 || result.stdout != "" {
			t.Fatalf("invalid arguments were not rejected: %+v", result)
		}
	})
	t.Run("malformed-passport-json", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "malformed-passport.json")
		if err := os.WriteFile(path, []byte(`{"schema_version":`), 0600); err != nil {
			t.Fatal(err)
		}
		result := runC20CLI(t, binary, "verify", "--passport", path)
		if result.code != 2 || result.stdout != "" {
			t.Fatalf("malformed passport was not rejected: %+v", result)
		}
	})

	t.Run("source-like-input-stays-inert", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "executed")
		data, err := os.ReadFile(input)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, []byte("synthetic Attestara maintainer"), []byte("touch "+marker), 1)
		path := filepath.Join(t.TempDir(), "source-like.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		result := runC20CLI(t, binary, "evaluate", "--input", path, "--policy", rule, "--evidence", evidence, "--freshness", "fresh")
		if result.code != 0 {
			t.Fatalf("source-like input failed evaluation: %+v", result)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("source-like input executed: %v", err)
		}
	})

	t.Run("engine-output-failure", func(t *testing.T) {
		full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		if err != nil {
			t.Skip("/dev/full is unavailable on this host")
		}
		defer full.Close()
		command := exec.Command(binary, "verify", "--passport", "fixtures/attestara/C20/expected.json")
		command.Dir = "."
		command.Stdout = full
		var stderr bytes.Buffer
		command.Stderr = &stderr
		err = command.Run()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 3 {
			t.Fatalf("output failure exit=%v, stderr=%s", err, stderr.String())
		}
	})
}

func TestC20FixtureProvenance(t *testing.T) {
	var positive struct {
		SchemaVersion int    `json:"schema_version"`
		License       string `json:"license"`
		Cases         []struct {
			ID string `json:"id"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c20Read(t, "positive.json"), &positive); err != nil {
		t.Fatal(err)
	}
	if positive.SchemaVersion != 1 || positive.License != "Apache-2.0" || len(positive.Cases) != 2 || positive.Cases[0].ID != "evaluate-pass" || positive.Cases[1].ID != "verify-and-export-passport" {
		t.Fatal("invalid C20 positive fixture contract")
	}
	var negative struct {
		SchemaVersion int    `json:"schema_version"`
		License       string `json:"license"`
		Cases         []struct {
			ID string `json:"id"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c20Read(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if negative.SchemaVersion != 1 || negative.License != "Apache-2.0" || len(negative.Cases) != 12 || negative.Cases[11].ID != "source-like-input" {
		t.Fatal("invalid C20 negative fixture contract")
	}
	var provenance struct {
		SchemaVersion int               `json:"schema_version"`
		License       string            `json:"license"`
		Files         map[string]string `json:"files"`
	}
	if err := json.Unmarshal(c20Read(t, "provenance.json"), &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.SchemaVersion != 1 || provenance.License != "Apache-2.0" {
		t.Fatal("invalid C20 fixture provenance")
	}
	entries, err := os.ReadDir("fixtures/attestara/C20")
	if err != nil {
		t.Fatal(err)
	}
	if len(provenance.Files) != len(entries)-1+7 {
		t.Fatalf("provenance covers %d files; want C20 fixtures plus seven reused C14 inputs", len(provenance.Files))
	}
	for _, entry := range entries {
		if entry.Name() == "provenance.json" {
			continue
		}
		data := c20Read(t, entry.Name())
		if provenance.Files["C20/"+entry.Name()] != fmt.Sprintf("sha256:%x", sha256.Sum256(data)) {
			t.Fatalf("C20 fixture hash mismatch: %s", entry.Name())
		}
	}
	for _, name := range []string{"input.json", "input.fail.json", "input.unknown.json", "rule.json", "evidence.json", "evidence.fail.json", "evidence.unknown.json"} {
		data, err := os.ReadFile(filepath.Join("fixtures/attestara/C14", name))
		if err != nil {
			t.Fatal(err)
		}
		if provenance.Files["C14/"+name] != fmt.Sprintf("sha256:%x", sha256.Sum256(data)) {
			t.Fatalf("reused C14 fixture hash mismatch: %s", name)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func reflectPassportEqual(a, b report.Passport) bool { return reflect.DeepEqual(a, b) }
