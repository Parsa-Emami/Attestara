package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/verify"
)

// These labels describe test scenarios, not a new production trust contract.
func TestC26BundleTrustMatrix(t *testing.T) {
	data, err := os.ReadFile("fixtures/attestara/C26/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version int `json:"schema_version"`
		Cases   []struct {
			ID                 string `json:"id"`
			Mutation           string `json:"mutation"`
			ArchiveCode        string `json:"archive_code"`
			ArchiveField       string `json:"archive_field"`
			SignatureCode      string `json:"signature_code"`
			CryptographicValid bool   `json:"cryptographic_valid"`
			SignerTrusted      bool   `json:"signer_trusted"`
			BundleTrusted      bool   `json:"bundle_trusted"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 11 {
		t.Fatal("unsupported or incomplete C26 matrix")
	}
	seen := map[string]bool{}
	for _, tc := range fixture.Cases {
		if seen[tc.ID] || tc.ID == "" {
			t.Fatal("duplicate or missing case identity")
		}
		seen[tc.ID] = true
		t.Run(tc.ID, func(t *testing.T) {
			manifest, signer, key, signature := c16Inputs(t)
			policy := verify.TrustPolicy{signer: bytes.Clone(key)}
			files := map[string][]byte{}
			for _, name := range []string{"subject.txt", "evidence.txt", "policy.txt"} {
				files[name] = readC08(t, name)
			}
			switch tc.Mutation {
			case "none":
			case "unsigned":
				signature = nil
			case "untrusted":
				policy = nil
			case "wrong-policy-key":
				policy[signer] = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32)).Public().(ed25519.PublicKey)
			case "forged-signature":
				signature[0] ^= 1
			case "manifest-whitespace":
				manifest = append(bytes.Clone(manifest), '\n')
			case "modified-subject":
				files["subject.txt"][0] ^= 1
			case "modified-evidence":
				files["evidence.txt"][0] ^= 1
			case "modified-policy":
				files["policy.txt"][0] ^= 1
			case "missing-evidence":
				delete(files, "evidence.txt")
			case "malformed-signature":
				signature = signature[:63]
			default:
				t.Fatal("unknown C26 mutation")
			}
			// Rebuild ZIP checksums: changed bytes must fail manifest integrity, not just CRC.
			archive := c26Archive(t, manifest, files)
			input, archiveErr := bundle.ReadArchive(archive)
			c26Error(t, archiveErr, tc.ArchiveCode, tc.ArchiveField)
			if archiveErr != nil && !reflect.DeepEqual(input, contracts.Input{}) {
				t.Fatal("rejected archive returned partial input")
			}
			if archiveErr == nil && input.Verification != "unverified" {
				t.Fatal("reader promoted input trust")
			}
			// Assess independently to prove that a valid signature cannot excuse missing/changed files.
			assessment, signatureErr := verify.VerifySignature(context.Background(), manifest, signer, key, signature, policy)
			c26Error(t, signatureErr, tc.SignatureCode, "")
			trusted := archiveErr == nil && signatureErr == nil && assessment.Trusted()
			if assessment.CryptographicValid != tc.CryptographicValid || assessment.SignerTrusted != tc.SignerTrusted || trusted != tc.BundleTrusted {
				t.Fatalf("signature=%+v bundleTrusted=%v; expected %+v", assessment, trusted, tc)
			}
			if !bytes.Equal(archive, c26Archive(t, manifest, files)) {
				t.Fatal("archive reader mutated caller bytes")
			}
		})
	}
}

func c26Error(t *testing.T, err error, code, field string) {
	t.Helper()
	if code == "" {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var typed *contracts.Error
	if !errors.As(err, &typed) || typed.Code != code || (field != "" && typed.Field != field) {
		t.Fatalf("error=%v; want %s/%s", err, code, field)
	}
}

func c26Archive(t *testing.T, manifest []byte, files map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	names := []string{"manifest.json", "subject.txt", "evidence.txt", "policy.txt", "source.sh"}
	for _, name := range names {
		data, exists := files[name]
		if name == "manifest.json" {
			data, exists = manifest, true
		}
		if !exists {
			continue
		}
		member, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := member.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestC26SignedArchiveSourceStaysInert(t *testing.T) {
	manifest, signer, key, signature := c16Inputs(t)
	sentinel := filepath.Join(t.TempDir(), "must-not-exist")
	files := map[string][]byte{"source.sh": []byte("#!/bin/sh\ntouch '" + sentinel + "'\n")}
	for _, name := range []string{"subject.txt", "evidence.txt", "policy.txt"} {
		files[name] = readC08(t, name)
	}
	if _, err := bundle.ReadArchive(c26Archive(t, manifest, files)); err != nil {
		t.Fatal(err)
	}
	got, err := verify.VerifySignature(context.Background(), manifest, signer, key, signature, verify.TrustPolicy{signer: key})
	if err != nil || !got.Trusted() {
		t.Fatalf("signature=%+v error=%v", got, err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("archive source executed", err)
	}
}

func TestC26FixtureProvenance(t *testing.T) {
	data, err := os.ReadFile("fixtures/attestara/C26/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Version int               `json:"schema_version"`
		License string            `json:"license"`
		Files   map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || fixture.License != "Apache-2.0" || len(fixture.Files) != 6 {
		t.Fatal("missing C26 provenance")
	}
	for name, expected := range fixture.Files {
		if !contracts.ValidManifestPath(name) {
			t.Fatal("unsafe fixture reference")
		}
		content, err := os.ReadFile(filepath.Join("fixtures/attestara", name))
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("sha256:%x", sha256.Sum256(content)); got != expected {
			t.Fatalf("%s: %s != %s", name, got, expected)
		}
	}
}
