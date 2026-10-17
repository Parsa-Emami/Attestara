package contracts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const fixtures = "../../tests/fixtures/attestara/C04"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func errorCode(t *testing.T, err error, want string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != want {
		t.Fatalf("error = %v; want %s", err, want)
	}
	if e.Message == "" {
		t.Fatal("empty safe error message")
	}
}
func encode(t *testing.T, value any) []byte {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func inputMap(t *testing.T) map[string]any {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(fixture(t, "input.valid.json"), &v); e != nil {
		t.Fatal(e)
	}
	return v
}

func TestC04Fixtures(t *testing.T) {
	input, err := DecodeInput(context.Background(), fixture(t, "input.valid.json"))
	if err != nil {
		t.Fatal(err)
	}
	if input.ReleaseID != "attestara-c04-release" || input.Verification != "unverified" {
		t.Fatal(input)
	}
	for _, f := range input.Files {
		b := fixture(t, f.Path)
		if fmt.Sprintf("sha256:%x", sha256.Sum256(b)) != f.Digest || int64(len(b)) != *f.SizeBytes {
			t.Fatalf("artifact mismatch %s", f.Path)
		}
	}
	for _, name := range []string{"result.valid.json", "result.failed.json"} {
		result, err := DecodeResult(context.Background(), fixture(t, name))
		if err != nil {
			t.Fatal(err)
		}
		if result.InputDigest != fmt.Sprintf("sha256:%x", sha256.Sum256(fixture(t, "input.valid.json"))) {
			t.Fatal("wrong input binding")
		}
		if result.Result != nil && (result.Result.SubjectDigest != input.SubjectDigest || result.Result.EvidenceDigest != input.EvidenceDigest || result.Result.PolicyDigest != input.PolicyDigest) {
			t.Fatal("wrong output binding")
		}
	}
	var negative struct {
		Cases []struct {
			ID, Target   string
			Value        json.RawMessage
			ExpectedCode string `json:"expected_code"`
		}
	}
	if err := json.Unmarshal(fixture(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) < 20 {
		t.Fatal("negative corpus missing")
	}
	for _, c := range negative.Cases {
		t.Run(c.ID, func(t *testing.T) {
			var err error
			if c.Target == "input" {
				_, err = DecodeInput(context.Background(), c.Value)
			} else {
				_, err = DecodeResult(context.Background(), c.Value)
			}
			errorCode(t, err, c.ExpectedCode)
		})
	}
}

func TestC04StrictJSON(t *testing.T) {
	for _, base := range []string{"input.valid.json", "result.valid.json", "result.failed.json"} {
		decode := func(b []byte) error {
			if base == "input.valid.json" {
				_, e := DecodeInput(context.Background(), b)
				return e
			}
			_, e := DecodeResult(context.Background(), b)
			return e
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(fixture(t, base), &obj); err != nil {
			t.Fatal(err)
		}
		for k := range obj {
			for _, null := range []bool{false, true} {
				v := map[string]json.RawMessage{}
				for a, b := range obj {
					v[a] = b
				}
				if null {
					v[k] = json.RawMessage("null")
				} else {
					delete(v, k)
				}
				// error and result are optional only on their corresponding result variants.
				if err := decode(encode(t, v)); err == nil {
					t.Fatalf("accepted missing/null %s in %s", k, base)
				}
			}
		}
		for _, b := range [][]byte{[]byte("null"), []byte("[]"), []byte("{"), append(fixture(t, base), []byte(" {}")...), []byte(`{"schema_version":"1","schema_version":"0"}`), {0xff}} {
			if err := decode(b); err == nil {
				t.Fatalf("accepted malformed JSON %q", b)
			}
		}
	}
	v := inputMap(t)
	v["SCHEMA_VERSION"] = v["schema_version"]
	delete(v, "schema_version")
	_, err := DecodeInput(context.Background(), encode(t, v))
	errorCode(t, err, "invalid_input")
	// Escaped duplicate names must be rejected before interpretation.
	_, err = DecodeInput(context.Background(), []byte(`{"schema_version":"1","schema_\u0076ersion":"1"}`))
	errorCode(t, err, "invalid_json")
	for _, raw := range []string{`{"schema_version":"1","files":[{"path":"a","size_bytes":1,"size_bytes":2}]}`, `{"schema_version":"1","files":[{"path":"a","digest":true}]}`} {
		if _, err := DecodeInput(context.Background(), []byte(raw)); err == nil {
			t.Fatal("accepted hostile nested JSON")
		}
	}
}

func TestC04LimitsAndCancellation(t *testing.T) {
	v := inputMap(t)
	files := v["files"].([]any)
	var total int64
	for _, f := range files[1:] {
		total += int64(f.(map[string]any)["size_bytes"].(float64))
	}
	files[0].(map[string]any)["size_bytes"] = MaxBundleBytes - total
	if _, err := DecodeInput(context.Background(), encode(t, v)); err != nil {
		t.Fatal("exact size cap", err)
	}
	files[0].(map[string]any)["size_bytes"] = MaxBundleBytes - total + 1
	_, err := DecodeInput(context.Background(), encode(t, v))
	errorCode(t, err, "limit_exceeded")
	v = inputMap(t)
	files = v["files"].([]any)
	for n := len(files); n < MaxFiles; n++ {
		files = append(files, map[string]any{"path": fmt.Sprintf("extra/%d", n), "digest": v["subject_digest"], "size_bytes": 0, "media_type": "application/octet-stream"})
	}
	v["files"] = files
	if _, err := DecodeInput(context.Background(), encode(t, v)); err != nil {
		t.Fatal("exact file cap", err)
	}
	v["files"] = append(files, files[0])
	_, err = DecodeInput(context.Background(), encode(t, v))
	errorCode(t, err, "limit_exceeded")
	for _, decode := range []func(context.Context, []byte) error{
		func(c context.Context, b []byte) error { _, e := DecodeInput(c, b); return e },
		func(c context.Context, b []byte) error { _, e := DecodeResult(c, b); return e },
	} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		errorCode(t, decode(ctx, []byte("{}")), "cancelled")
		ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		errorCode(t, decode(ctx, []byte("{}")), "deadline_exceeded")
		cancel()
		oversized := bytes.Repeat([]byte(" "), int(MaxBundleBytes)+1)
		errorCode(t, decode(context.Background(), oversized), "limit_exceeded")
	}
	wire := bytes.Repeat([]byte(" "), int(MaxBundleBytes)+1)
	valid := fixture(t, "input.valid.json")
	copy(wire, valid)
	if _, err := DecodeInput(context.Background(), wire[:MaxBundleBytes]); err != nil {
		t.Fatal("exact JSON byte cap", err)
	}
	_, err = DecodeInput(context.Background(), wire)
	errorCode(t, err, "limit_exceeded")
	// Oversized manifests are rejected before typed slice allocation.
	v = inputMap(t)
	v["files"] = json.RawMessage("[" + strings.Repeat("null,", 49999) + "null]")
	_, err = DecodeInput(context.Background(), encode(t, v))
	errorCode(t, err, "limit_exceeded")
	if ValidationDeadline != 10*time.Second {
		t.Fatal("product deadline drift")
	}
}

func TestC04DraftCompatibility(t *testing.T) {
	for _, file := range []string{"input", "result"} {
		b, err := os.ReadFile("../../contracts/drafts/" + file + ".v0.schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "draft") {
			t.Fatal("draft lost")
		}
	}
	// C01 is historical specification data, never silently accepted as v1.
	b, err := os.ReadFile("../../tests/fixtures/attestara/C01/positive.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeInput(context.Background(), b); err == nil {
		t.Fatal("silently upgraded C01")
	}
}

func TestC04NestedFieldsResultsAndNoExecution(t *testing.T) {
	for _, field := range []string{"path", "digest", "size_bytes", "media_type"} {
		for _, null := range []bool{false, true} {
			v := inputMap(t)
			file := v["files"].([]any)[0].(map[string]any)
			if null {
				file[field] = nil
			} else {
				delete(file, field)
			}
			_, err := DecodeInput(context.Background(), encode(t, v))
			errorCode(t, err, "invalid_input")
		}
	}
	var result map[string]any
	if err := json.Unmarshal(fixture(t, "result.valid.json"), &result); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"complete", "partial", "unverified"} {
		for _, decision := range []string{"allow", "deny"} {
			result["status"] = status
			result["result"].(map[string]any)["policy_decision"] = decision
			got, err := DecodeResult(context.Background(), encode(t, result))
			if err != nil {
				t.Fatal(err)
			}
			if got.Result.Verification != "unverified" {
				t.Fatal("policy promoted authenticity")
			}
		}
	}
	for _, section := range []string{"result", "error"} {
		name := "result.valid.json"
		if section == "error" {
			name = "result.failed.json"
		}
		for _, field := range []string{"subject_digest", "evidence_digest", "policy_digest", "policy_decision", "verification", "code", "field", "message"} {
			var v map[string]any
			if err := json.Unmarshal(fixture(t, name), &v); err != nil {
				t.Fatal(err)
			}
			fields := v[section].(map[string]any)
			if _, ok := fields[field]; !ok {
				continue
			}
			delete(fields, field)
			_, err := DecodeResult(context.Background(), encode(t, v))
			errorCode(t, err, "invalid_input")
		}
	}
	for _, n := range []int{4096, 4097} {
		v := inputMap(t)
		v["source_revision"] = strings.Repeat("界", n)
		_, err := DecodeInput(context.Background(), encode(t, v))
		if n == 4096 && err != nil {
			t.Fatal(err)
		}
		if n == 4097 {
			errorCode(t, err, "invalid_input")
		}
	}
	marker := filepath.Join(t.TempDir(), "executed")
	v := inputMap(t)
	v["tool"] = "$(touch " + marker + ")"
	if _, err := DecodeInput(context.Background(), encode(t, v)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("user source was executed")
	}
	v["extra_secret_marker"] = "secret-content-must-not-appear"
	_, err := DecodeInput(context.Background(), encode(t, v))
	errorCode(t, err, "invalid_input")
	if strings.Contains(err.Error(), "secret") {
		t.Fatal("error leaked input")
	}
}
