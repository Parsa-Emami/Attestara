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
	"strings"
	"testing"
	"time"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/verify"
)

func c17Read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C17", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestC17SubjectBinding(t *testing.T) {
	var expected struct {
		Cases []struct {
			Mutation     string
			SubjectBound bool `json:"subject_bound"`
		}
		Verification string
	}
	if err := json.Unmarshal(c17Read(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	if len(expected.Cases) != 2 || expected.Verification != "unverified" {
		t.Fatal("missing expected outcomes")
	}
	for _, tc := range expected.Cases {
		t.Run(tc.Mutation, func(t *testing.T) {
			manifest, envelope := readC08(t, "positive.json"), c17Read(t, "positive.json")
			if tc.Mutation == "different-subject" {
				envelope = bytes.Replace(envelope, []byte("sha256:932b"), []byte("sha256:832b"), 1)
			}
			got, err := verify.VerifyProvenance(context.Background(), manifest, envelope)
			if err != nil || got.SubjectBound != tc.SubjectBound {
				t.Fatalf("got %+v / %v; want bound=%v", got, err, tc.SubjectBound)
			}
			input, err := contracts.DecodeInput(context.Background(), manifest)
			if err != nil || input.Verification != "unverified" {
				t.Fatal("binding promoted verification", err)
			}
		})
	}
}

func TestC17InvalidInputsAndCancellation(t *testing.T) {
	var negative struct {
		Cases []struct{ Mutation, Code string }
	}
	if err := json.Unmarshal(c17Read(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) != 11 {
		t.Fatal("missing negative cases")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.Mutation, func(t *testing.T) {
			manifest, envelope := readC08(t, "positive.json"), c17Read(t, "positive.json")
			ctx := context.Background()
			switch tc.Mutation {
			case "missing-envelope":
				envelope = nil
			case "null-envelope":
				envelope = []byte("null")
			case "missing-subject":
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(envelope, &fields); err != nil {
					t.Fatal(err)
				}
				delete(fields, "subject_digest")
				var err error
				envelope, err = json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
			case "bad-digest":
				envelope = bytes.Replace(envelope, []byte("sha256:"), []byte("sha512:"), 1)
			case "unknown-field":
				envelope = bytes.Replace(envelope, []byte(`"schema_version"`), []byte(`"secret":"synthetic-secret","schema_version"`), 1)
			case "duplicate-field":
				envelope = bytes.Replace(envelope, []byte(`"schema_version"`), []byte(`"schema_version":"1","schema_version"`), 1)
			case "unsupported-version":
				envelope = bytes.Replace(envelope, []byte(`"schema_version": "1"`), []byte(`"schema_version": "2"`), 1)
			case "oversized-envelope":
				envelope = make([]byte, contracts.MaxBundleBytes+1)
			case "missing-manifest":
				manifest = nil
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "deadline":
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Unix(0, 0))
				defer cancel()
			default:
				t.Fatal("unknown fixture mutation")
			}
			got, err := verify.VerifyProvenance(ctx, manifest, envelope)
			var typed *contracts.Error
			if got != (verify.ProvenanceVerification{}) || !errors.As(err, &typed) || typed.Code != tc.Code || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatalf("got %+v / %v; want %s", got, err, tc.Code)
			}
		})
	}
}

func TestC17IndependentTrustFilesAndStatelessness(t *testing.T) {
	manifest, id, key, sig := c16Inputs(t)
	envelope := c17Read(t, "positive.json")
	originalManifest, originalEnvelope := bytes.Clone(manifest), bytes.Clone(envelope)
	for i := 0; i < 3; i++ {
		got, err := verify.VerifyProvenance(context.Background(), manifest, envelope)
		if err != nil || !got.SubjectBound {
			t.Fatalf("repeat: %+v / %v", got, err)
		}
	}
	signature, err := verify.VerifySignature(context.Background(), manifest, id, key, sig, nil)
	if err != nil || !signature.CryptographicValid || signature.Trusted() {
		t.Fatal("subject binding granted signer trust", err)
	}
	root := copyC08(t)
	if _, err := bundle.ReadRoot(manifest, openC08Root(t, root)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "evidence.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.ReadRoot(manifest, openC08Root(t, root)); err == nil {
		t.Fatal("missing actual evidence accepted")
	}
	// Subject binding remains a claim comparison even if referenced files are missing.
	if got, err := verify.VerifyProvenance(context.Background(), manifest, envelope); err != nil || !got.SubjectBound {
		t.Fatal(got, err)
	}
	var input contracts.Input
	if err := json.Unmarshal(manifest, &input); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(t.TempDir(), "must-not-exist")
	input.SourceRevision = "$(touch " + sentinel + ") <script>inert</script>"
	inert, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := verify.VerifyProvenance(context.Background(), inert, envelope); err != nil || !got.SubjectBound {
		t.Fatal(got, err)
	}
	if _, err := os.Stat(sentinel); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source-like text executed", err)
	}
	if !bytes.Equal(manifest, originalManifest) || !bytes.Equal(envelope, originalEnvelope) {
		t.Fatal("inputs mutated")
	}
}

func TestC17FixtureProvenance(t *testing.T) {
	var p struct {
		SchemaVersion int `json:"schema_version"`
		License       string
		Files         map[string]string
		ReusedFiles   map[string]string `json:"reused_files"`
	}
	if err := json.Unmarshal(c17Read(t, "provenance.json"), &p); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("fixtures/attestara/C17")
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 1 || p.License != "Apache-2.0" || len(p.Files) != len(entries)-1 || len(p.ReusedFiles) != 4 {
		t.Fatal("incomplete provenance")
	}
	for _, entry := range entries {
		if entry.Name() != "provenance.json" && (entry.IsDir() || p.Files[entry.Name()] != fmt.Sprintf("sha256:%x", sha256.Sum256(c17Read(t, entry.Name())))) {
			t.Fatal("fixture hash differs", entry.Name())
		}
	}
	for path, want := range p.ReusedFiles {
		b, err := os.ReadFile(filepath.Join("..", path))
		if err != nil || fmt.Sprintf("sha256:%x", sha256.Sum256(b)) != want {
			t.Fatal("reused fixture changed", path, err)
		}
	}
}
