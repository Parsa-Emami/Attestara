package tests

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/verify"
)

func c16Read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C16", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func c16Inputs(t *testing.T) ([]byte, string, ed25519.PublicKey, []byte) {
	t.Helper()
	var f struct {
		SignerID    string `json:"signer_id"`
		PublicKey   string `json:"public_key_hex"`
		Signature   string `json:"signature_hex"`
		PayloadHash string `json:"payload_sha256"`
	}
	if err := json.Unmarshal(c16Read(t, "positive.json"), &f); err != nil {
		t.Fatal(err)
	}
	key, err := hex.DecodeString(f.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := hex.DecodeString(f.Signature)
	if err != nil {
		t.Fatal(err)
	}
	payload := readC08(t, "positive.json")
	if fmt.Sprintf("sha256:%x", sha256.Sum256(payload)) != f.PayloadHash {
		t.Fatal("signed payload changed")
	}
	return payload, f.SignerID, key, sig
}

func TestC16SignatureAndTrustMatrix(t *testing.T) {
	var f struct {
		Cases []struct {
			ID, Mutation       string
			CryptographicValid bool `json:"cryptographic_valid"`
			SignerTrusted      bool `json:"signer_trusted"`
			Trusted            bool
		}
	}
	if err := json.Unmarshal(c16Read(t, "expected.json"), &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 8 {
		t.Fatal("missing matrix cases")
	}
	for _, tc := range f.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			payload, id, key, sig := c16Inputs(t)
			policy := verify.TrustPolicy{id: bytes.Clone(key)}
			switch tc.Mutation {
			case "none":
			case "empty-policy":
				policy = nil
			case "wrong-identity":
				id = "untrusted-synthetic-identity"
			case "wrong-policy-key":
				policy[id] = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32)).Public().(ed25519.PublicKey)
			case "modified-byte":
				payload = bytes.Replace(payload, []byte("revision-1"), []byte("revision-0"), 1)
			case "invalid-signature":
				sig[0] ^= 1
			case "wrong-key":
				key = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{42}, 32)).Public().(ed25519.PublicKey)
			case "unsigned":
				sig = nil
			default:
				t.Fatal("unknown mutation")
			}
			got, err := verify.VerifySignature(context.Background(), payload, id, key, sig, policy)
			if err != nil || got.CryptographicValid != tc.CryptographicValid || got.SignerTrusted != tc.SignerTrusted || got.Trusted() != tc.Trusted {
				t.Fatalf("assessment=%+v / %v; expected=%+v", got, err, tc)
			}
		})
	}
}

func TestC16MalformedInputsAndCancellation(t *testing.T) {
	var f struct {
		Cases []struct{ Mutation, Code string }
	}
	if err := json.Unmarshal(c16Read(t, "negative.json"), &f); err != nil {
		t.Fatal(err)
	}
	for _, tc := range f.Cases {
		t.Run(tc.Mutation, func(t *testing.T) {
			p, id, k, s := c16Inputs(t)
			policy := verify.TrustPolicy{id: bytes.Clone(k)}
			ctx := context.Background()
			switch tc.Mutation {
			case "short-key":
				k = k[:31]
			case "long-key":
				k = append(k, 0)
			case "short-signature":
				s = s[:63]
			case "long-signature":
				s = append(s, 0)
			case "bad-policy-key":
				policy[id] = []byte("synthetic-secret")
			case "empty-signer":
				id = ""
			case "invalid-signer-utf8":
				id = string([]byte{255})
			case "oversized-signer":
				id = strings.Repeat("x", 4097)
			case "missing-manifest":
				p = nil
			case "unknown-field":
				p = bytes.Replace(p, []byte(`"schema_version"`), []byte(`"secret": "synthetic-secret", "schema_version"`), 1)
			case "duplicate-field":
				p = bytes.Replace(p, []byte(`"schema_version"`), []byte(`"schema_version":"1", "schema_version"`), 1)
			case "oversized-manifest":
				p = make([]byte, contracts.MaxBundleBytes+1)
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "deadline":
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Unix(0, 0))
				defer cancel()
			default:
				t.Fatal("unknown negative")
			}
			got, err := verify.VerifySignature(ctx, p, id, k, s, policy)
			var typed *contracts.Error
			if got != (verify.SignatureVerification{}) || !errors.As(err, &typed) || typed.Code != tc.Code || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatalf("got=%+v err=%v; want %s", got, err, tc.Code)
			}
		})
	}
}

func TestC16RealBundleBoundary(t *testing.T) {
	p, id, k, s := c16Inputs(t)
	policy := verify.TrustPolicy{id: k}
	root := copyC08(t)
	assess := func() (verify.SignatureVerification, error) {
		if _, err := bundle.ReadRoot(p, openC08Root(t, root)); err != nil {
			return verify.SignatureVerification{}, err
		}
		return verify.VerifySignature(context.Background(), p, id, k, s, policy)
	}
	got, err := assess()
	if err != nil || !got.Trusted() {
		t.Fatalf("real bundle: %+v / %v", got, err)
	}
	for _, name := range []string{"subject.txt", "evidence.txt", "policy.txt"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := bytes.Clone(original)
			changed[0] ^= 1
			if err := os.WriteFile(path, changed, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := assess(); err == nil || got.Trusted() {
				t.Fatalf("modified file trusted: %+v / %v", got, err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if got, err := assess(); err == nil || got.Trusted() {
				t.Fatalf("missing file trusted: %+v / %v", got, err)
			}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestC16StatelessRepeatAndInertBytes(t *testing.T) {
	p, id, k, s := c16Inputs(t)
	policy := verify.TrustPolicy{id: bytes.Clone(k)}
	p0, k0, s0 := bytes.Clone(p), bytes.Clone(k), bytes.Clone(s)
	whitespace, err := verify.VerifySignature(context.Background(), append(bytes.Clone(p), '\n'), id, k, s, policy)
	if err != nil || whitespace.Trusted() || whitespace.CryptographicValid {
		t.Fatalf("whitespace reserialization authenticated: %+v / %v", whitespace, err)
	}
	revoked, err := verify.VerifySignature(context.Background(), p, id, k, s, nil)
	if err != nil || !revoked.CryptographicValid || revoked.SignerTrusted || revoked.Trusted() {
		t.Fatalf("revoked policy retained trust: %+v / %v", revoked, err)
	}

	first, err := verify.VerifySignature(context.Background(), p, id, k, s, policy)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		got, err := verify.VerifySignature(context.Background(), p, id, k, s, policy)
		if err != nil || got != first {
			t.Fatalf("repeat: %+v / %v", got, err)
		}
	}
	if !bytes.Equal(p, p0) || !bytes.Equal(k, k0) || !bytes.Equal(s, s0) || !bytes.Equal(policy[id], k0) {
		t.Fatal("caller inputs mutated")
	}
	// A synthetically signed inert manifest can contain source-like text; verification must never execute it.
	sentinel := filepath.Join(t.TempDir(), "must-not-exist")
	var manifest contracts.Input
	if err := json.Unmarshal(p, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.SourceRevision = "$(touch " + sentinel + ") <script>inert</script>"
	inert, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	priv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32))
	pub := priv.Public().(ed25519.PublicKey)
	got, err := verify.VerifySignature(context.Background(), inert, id, pub, ed25519.Sign(priv, inert), verify.TrustPolicy{id: pub})
	if err != nil || !got.Trusted() {
		t.Fatalf("inert payload: %+v / %v", got, err)
	}
	if _, err := os.Stat(sentinel); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("input executed")
	}
	if !reflect.DeepEqual(policy, verify.TrustPolicy{id: k0}) {
		t.Fatal("policy changed")
	}
}

func TestC16FixtureProvenance(t *testing.T) {
	var f struct {
		SchemaVersion   int `json:"schema_version"`
		License, Origin string
		Files           map[string]string
	}
	if err := json.Unmarshal(c16Read(t, "provenance.json"), &f); err != nil {
		t.Fatal(err)
	}
	if f.SchemaVersion != 1 || f.License != "Apache-2.0" || f.Origin == "" || len(f.Files) != 4 {
		t.Fatal("missing provenance")
	}
	for name, want := range f.Files {
		if got := fmt.Sprintf("sha256:%x", sha256.Sum256(c16Read(t, name))); got != want {
			t.Fatalf("%s checksum mismatch", name)
		}
	}
}
