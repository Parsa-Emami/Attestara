package adapters

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"strings"
	"time"

	"attestara/internal/contracts"
)

// VulnerabilityReport is an inert projection of the documented Grype JSON
// presenter. Database and feed status are claims in the report, not trust proof.
type VulnerabilityReport struct {
	ScannerVersion     string                 `json:"scanner_version"`
	ScanTime           string                 `json:"scan_time,omitempty"`
	DatabaseSchema     string                 `json:"database_schema,omitempty"`
	DatabaseBuilt      string                 `json:"database_built,omitempty"`
	DatabaseAgeSeconds *int64                 `json:"database_age_seconds,omitempty"`
	DatabaseStatus     string                 `json:"database_status"`
	CVEFeedCaptured    string                 `json:"cve_feed_captured,omitempty"`
	CVEFeedStatus      string                 `json:"cve_feed_status"`
	Findings           []VulnerabilityFinding `json:"findings"`
}

type VulnerabilityFinding struct {
	VulnerabilityID string `json:"vulnerability_id"`
	Severity        string `json:"severity,omitempty"`
	ArtifactID      string `json:"artifact_id"`
	ArtifactName    string `json:"artifact_name"`
	ArtifactVersion string `json:"artifact_version,omitempty"`
}

type grypeDocument struct {
	Matches []struct {
		Vulnerability struct {
			ID       string `json:"id"`
			Severity string `json:"severity"`
		} `json:"vulnerability"`
		Artifact struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"artifact"`
	} `json:"matches"`
	Descriptor struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		Timestamp string `json:"timestamp"`
		DB        *struct {
			Status *struct {
				SchemaVersion string `json:"schemaVersion"`
				Built         string `json:"built"`
				Valid         *bool  `json:"valid"`
			} `json:"status"`
			Providers map[string]struct {
				Captured string `json:"captured"`
			} `json:"providers"`
		} `json:"db"`
	} `json:"descriptor"`
}

const maxGrypeMatches = 10000

func grypeFailure(code, field string) error {
	return &contracts.Error{Code: code, Field: field, Message: "Grype JSON report is missing or has an invalid supported field."}
}

func grypeText(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 4096 }

// DecodeGrypeReport reads an existing Grype JSON report without invoking a
// scanner, fetching a feed, retaining configuration, or persisting input.
func DecodeGrypeReport(data []byte) (VulnerabilityReport, error) {
	fail := func(code, field string) (VulnerabilityReport, error) {
		return VulnerabilityReport{}, grypeFailure(code, field)
	}
	if int64(len(data)) > contracts.MaxBundleBytes {
		return fail("limit_exceeded", "$")
	}
	if !jsontext.Value(data).IsValid() {
		return fail("invalid_json", "$")
	}
	var root map[string]jsontext.Value
	if json.Unmarshal(data, &root) != nil || root == nil {
		return fail("invalid_input", "$")
	}
	if !bytes.HasPrefix(bytes.TrimSpace(root["matches"]), []byte("[")) {
		return fail("invalid_input", "matches")
	}
	// Count before unmarshalling the typed matches and allocating the projection.
	matches := jsontext.NewDecoder(bytes.NewReader(root["matches"]))
	if _, err := matches.ReadToken(); err != nil {
		return fail("invalid_json", "matches")
	}
	for count := 0; matches.PeekKind() != ']'; count++ {
		if count >= maxGrypeMatches {
			return fail("limit_exceeded", "matches")
		}
		if err := matches.SkipValue(); err != nil {
			return fail("invalid_json", "matches")
		}
	}
	var doc grypeDocument
	if json.Unmarshal(data, &doc) != nil {
		return fail("invalid_input", "$")
	}
	if doc.Descriptor.Name != "grype" {
		return fail("invalid_input", "descriptor.name")
	}
	if !grypeText(doc.Descriptor.Version) {
		return fail("invalid_input", "descriptor.version")
	}
	out := VulnerabilityReport{
		ScannerVersion: doc.Descriptor.Version,
		ScanTime:       doc.Descriptor.Timestamp,
		DatabaseStatus: "unknown",
		CVEFeedStatus:  "unknown",
		Findings:       make([]VulnerabilityFinding, 0, len(doc.Matches)),
	}
	var scanTime time.Time
	if out.ScanTime != "" {
		var err error
		scanTime, err = time.Parse(time.RFC3339, out.ScanTime)
		if err != nil {
			return fail("invalid_input", "descriptor.timestamp")
		}
	}
	if db := doc.Descriptor.DB; db != nil {
		if status := db.Status; status != nil {
			out.DatabaseSchema = status.SchemaVersion
			out.DatabaseBuilt = status.Built
			if status.Valid != nil {
				out.DatabaseStatus = "reported_invalid"
				if *status.Valid {
					out.DatabaseStatus = "reported_valid"
				}
			}
			if status.Built != "" {
				built, err := time.Parse(time.RFC3339, status.Built)
				if err != nil {
					return fail("invalid_input", "descriptor.db.status.built")
				}
				if !scanTime.IsZero() && !built.After(scanTime) {
					age := scanTime.Unix() - built.Unix()
					out.DatabaseAgeSeconds = &age
				}
			}
		}
		if nvd, ok := db.Providers["nvd"]; ok && nvd.Captured != "" {
			if _, err := time.Parse(time.RFC3339, nvd.Captured); err != nil {
				return fail("invalid_input", "descriptor.db.providers.nvd.captured")
			}
			out.CVEFeedCaptured = nvd.Captured
			if out.DatabaseStatus == "reported_valid" {
				out.CVEFeedStatus = "reported"
			}
		}
	}
	for i, match := range doc.Matches {
		for _, field := range []struct{ name, value string }{
			{"vulnerability.id", match.Vulnerability.ID},
			{"artifact.id", match.Artifact.ID},
			{"artifact.name", match.Artifact.Name},
		} {
			if !grypeText(field.value) {
				return fail("invalid_input", fmt.Sprintf("matches[%d].%s", i, field.name))
			}
		}
		out.Findings = append(out.Findings, VulnerabilityFinding{
			VulnerabilityID: match.Vulnerability.ID,
			Severity:        match.Vulnerability.Severity,
			ArtifactID:      match.Artifact.ID,
			ArtifactName:    match.Artifact.Name,
			ArtifactVersion: match.Artifact.Version,
		})
	}
	return out, nil
}
