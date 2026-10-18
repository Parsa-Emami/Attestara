package tests

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	jsonv2 "encoding/json/v2"

	"attestara/internal/contracts"
	"attestara/internal/decision"
	"attestara/internal/report"
)

func TestC25PublicPassportRedaction(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("fixtures", "attestara", "C25", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion int    `json:"schema_version"`
		License       string `json:"license"`
		Provenance    string `json:"provenance"`
		Cases         []struct {
			ID       string `json:"id"`
			Value    string `json:"value"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.License != "Apache-2.0" || fixture.Provenance == "" || len(fixture.Cases) != 14 {
		t.Fatal("invalid C25 fixture identity")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			input, result := c19Build(t, "pass", false, false, decision.Fresh)
			input.ReleaseID, input.SourceRevision = tc.Value, tc.Value
			original := input
			got, err := report.GeneratePassport(context.Background(), input, result)
			if err != nil {
				t.Fatal(err)
			}
			if got.ReleaseID != tc.Expected || got.SourceRevision != tc.Expected || !reflect.DeepEqual(input, original) {
				t.Fatalf("public text or input ownership changed: %+v", got)
			}
			payload := got
			payload.ReportDigest = ""
			encoded, err := jsonv2.Marshal(payload, jsonv2.FormatNilSliceAsNull(true))
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(encoded)
			if got.ReportDigest != "sha256:"+hex.EncodeToString(digest[:]) {
				t.Fatal("report digest does not bind redacted bytes")
			}
		})
	}
}

func TestC25AbsoluteEvidencePathIsRejected(t *testing.T) {
	for _, path := range []string{"/home/synthetic/private/evidence.json", `C:\Users\synthetic\evidence.json`} {
		t.Run(path, func(t *testing.T) {
			input, result := c19Build(t, "pass", false, false, decision.Fresh)
			input.Files[0].Path = path
			passport, err := report.GeneratePassport(context.Background(), input, result)
			var typed *contracts.Error
			if !reflect.DeepEqual(passport, report.Passport{}) || !errors.As(err, &typed) || typed.Code != "invalid_input" || typed.Field != "files.path" {
				t.Fatalf("absolute path entered public passport: %+v / %v", passport, err)
			}
		})
	}
}

func TestC25WarningsDoNotPublishFreeText(t *testing.T) {
	input, result := c19Build(t, "pass", false, false, decision.Fresh)
	result.Warnings = []string{decision.EvidenceMissing, decision.EvidenceStale, decision.PolicyFailed, "func syntheticSource() { synthetic-secret }", "Authorization: Bearer synthetic-token-value"}
	warnings := append([]string(nil), result.Warnings...)
	got, err := report.GeneratePassport(context.Background(), input, result)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{decision.EvidenceMissing, decision.EvidenceStale, decision.PolicyFailed, "[REDACTED]", "[REDACTED]"}
	if !reflect.DeepEqual(got.Warnings, want) || !reflect.DeepEqual(result.Warnings, warnings) {
		t.Fatalf("warning projection or input ownership changed: %v", got.Warnings)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"syntheticSource", "synthetic-secret", "synthetic-token-value"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("public passport leaked %q", secret)
		}
	}
}
