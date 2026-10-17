package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"attestara/internal/contracts"
	"attestara/internal/evidence"
)

const c07Fixtures = "fixtures/attestara/C07"

func readC07(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(c07Fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestC07EvidenceEnvelopeBindsSubjectToolConfigurationAndArtifact(t *testing.T) {
	want := struct {
		SchemaVersion       string `json:"schema_version"`
		SubjectDigest       string `json:"subject_digest"`
		Tool                string `json:"tool"`
		ToolVersion         string `json:"tool_version"`
		ConfigurationDigest string `json:"configuration_digest"`
		ArtifactDigest      string `json:"artifact_digest"`
	}{}
	if err := json.Unmarshal(readC07(t, "expected.json"), &want); err != nil {
		t.Fatal(err)
	}
	got, err := evidence.DecodeEnvelope(readC07(t, "positive.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != want.SchemaVersion || got.SubjectDigest != want.SubjectDigest || got.Tool != want.Tool || got.ToolVersion != want.ToolVersion || got.ConfigurationDigest != want.ConfigurationDigest || got.Artifact.Digest != want.ArtifactDigest {
		t.Fatalf("envelope = %+v; want fixture binding %+v", got, want)
	}
	for _, file := range []struct{ name, digest string }{
		{"subject.txt", got.SubjectDigest},
		{"configuration.json", got.ConfigurationDigest},
		{"evidence.txt", got.Artifact.Digest},
	} {
		b := readC07(t, file.name)
		sum := sha256.Sum256(b)
		if digest := "sha256:" + hex.EncodeToString(sum[:]); digest != file.digest {
			t.Errorf("%s digest = %s; envelope says %s", file.name, digest, file.digest)
		}
		if file.name == "evidence.txt" && int64(len(b)) != *got.Artifact.SizeBytes {
			t.Errorf("artifact size = %d; fixture says %d", len(b), *got.Artifact.SizeBytes)
		}
	}
}

func TestC07EnvelopeRejectsInvalidFixtures(t *testing.T) {
	var negative struct {
		Cases []struct {
			ID           string          `json:"id"`
			Field        string          `json:"field,omitempty"`
			Mutation     string          `json:"mutation,omitempty"`
			Value        json.RawMessage `json:"value,omitempty"`
			Raw          string          `json:"raw,omitempty"`
			ExpectedCode string          `json:"expected_code"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(readC07(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	var base map[string]json.RawMessage
	if err := json.Unmarshal(readC07(t, "positive.json"), &base); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) == 0 {
		t.Fatal("negative fixture has no cases")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			var input []byte
			if tc.Raw != "" {
				input = []byte(tc.Raw)
			} else {
				value := make(map[string]json.RawMessage, len(base))
				for key, raw := range base {
					value[key] = raw
				}
				switch tc.Mutation {
				case "remove":
					delete(value, tc.Field)
				case "set":
					value[tc.Field] = tc.Value
				case "unknown":
					value["unexpected"] = json.RawMessage(`true`)
				default:
					t.Fatalf("unknown fixture mutation %q", tc.Mutation)
				}
				var err error
				input, err = json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
			}
			_, err := evidence.DecodeEnvelope(input)
			var contractErr *contracts.Error
			if !errors.As(err, &contractErr) || contractErr.Code != tc.ExpectedCode || contractErr.Message == "" {
				t.Fatalf("error = %v; want safe %s error", err, tc.ExpectedCode)
			}
		})
	}
}

func TestC07EnvelopeReloadPreservesBindings(t *testing.T) {
	input := readC07(t, "positive.json")
	want, err := evidence.DecodeEnvelope(input)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "envelope.json")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := evidence.DecodeEnvelope(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded evidence binding changed: got %+v; want %+v", got, want)
	}
}
