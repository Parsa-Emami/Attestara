package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"attestara/internal/adapters"
	"attestara/internal/contracts"
	"attestara/internal/report"
)

func c22Read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C22", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type c22Snapshot struct {
	Passport report.Passport              `json:"passport"`
	Report   adapters.VulnerabilityReport `json:"vulnerability_report"`
}

type c22Fixture struct {
	ID            string      `json:"id"`
	Before        c22Snapshot `json:"before"`
	After         c22Snapshot `json:"after"`
	Expected      report.Diff `json:"expected"`
	SchemaVersion int         `json:"schema_version"`
	License       string      `json:"license"`
}

func TestC22DiffPositiveFixture(t *testing.T) {
	var fixture c22Fixture
	if err := json.Unmarshal(c22Read(t, "positive.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.License != "Apache-2.0" || fixture.ID != "independent-changes" {
		t.Fatal("missing versioned C22 positive fixture")
	}
	if fixture.Before.Passport.Verification != "unverified" || fixture.After.Passport.Verification != "unverified" {
		t.Fatal("fixture must retain the passports' unverified claims")
	}

	before, _ := json.Marshal(fixture.Before)
	after, _ := json.Marshal(fixture.After)
	got := report.Compare(fixture.Before.Passport, fixture.Before.Report, fixture.After.Passport, fixture.After.Report)
	if !reflect.DeepEqual(got, fixture.Expected) {
		t.Fatalf("diff mismatch\ngot:  %+v\nwant: %+v", got, fixture.Expected)
	}
	beforeAfter, _ := json.Marshal(fixture.Before)
	afterAfter, _ := json.Marshal(fixture.After)
	if !reflect.DeepEqual(before, beforeAfter) || !reflect.DeepEqual(after, afterAfter) {
		t.Fatal("diff changed an input snapshot")
	}
}

func TestC22DiffNegativeFixtures(t *testing.T) {
	var fixture struct {
		SchemaVersion int    `json:"schema_version"`
		License       string `json:"license"`
		Cases         []struct {
			ID     string `json:"id"`
			Before struct {
				EvidenceReferences []contracts.EvidenceReference   `json:"evidence_references"`
				Findings           []adapters.VulnerabilityFinding `json:"findings"`
			} `json:"before"`
			After struct {
				EvidenceReferences []contracts.EvidenceReference   `json:"evidence_references"`
				Findings           []adapters.VulnerabilityFinding `json:"findings"`
			} `json:"after"`
			Expected report.Diff `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(c22Read(t, "negative.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || fixture.License != "Apache-2.0" || len(fixture.Cases) != 6 {
		t.Fatal("missing versioned C22 negative fixtures")
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			before := report.Passport{Verification: "unverified", EvidenceReferences: tc.Before.EvidenceReferences}
			after := report.Passport{Verification: "unverified", EvidenceReferences: tc.After.EvidenceReferences}
			beforeReport := adapters.VulnerabilityReport{Findings: tc.Before.Findings}
			afterReport := adapters.VulnerabilityReport{Findings: tc.After.Findings}
			got := report.Compare(before, beforeReport, after, afterReport)
			if !reflect.DeepEqual(got, tc.Expected) {
				t.Fatalf("diff mismatch\ngot:  %+v\nwant: %+v", got, tc.Expected)
			}
		})
	}
}
