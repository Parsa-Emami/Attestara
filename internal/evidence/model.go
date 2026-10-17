// Package evidence models a single versioned artifact bound to its subject,
// producing tool and version, and configuration digest.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"strings"
	"unicode/utf8"

	"attestara/internal/contracts"
)

const SchemaVersion = "1"

// Envelope records one evidence artifact and the inputs that identify its
// provenance. File metadata reuses the existing C04 contract type.
type Envelope struct {
	SchemaVersion       string         `json:"schema_version"`
	SubjectDigest       string         `json:"subject_digest"`
	Tool                string         `json:"tool"`
	ToolVersion         string         `json:"tool_version"`
	ConfigurationDigest string         `json:"configuration_digest"`
	Artifact            contracts.File `json:"artifact"`
}

func failure(code, field, message string) error {
	if field == "" {
		field = "$"
	}
	return &contracts.Error{Code: code, Field: field, Message: message}
}

func invalid(field string) error {
	return failure("invalid_input", field, "Required value is missing or invalid.")
}

func validDigest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value[len(prefix):])
	return err == nil && hex.EncodeToString(decoded) == value[len(prefix):]
}

func validText(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 4096
}

// DecodeEnvelope strictly decodes a bounded envelope and validates the SHA-256
// format of each digest binding the evidence to its subject and configuration.
func DecodeEnvelope(data []byte) (Envelope, error) {
	if int64(len(data)) > contracts.MaxBundleBytes {
		return Envelope{}, failure("limit_exceeded", "$", "JSON exceeds the 100 MiB byte limit.")
	}
	var value Envelope
	if err := json.Unmarshal(data, &value, json.RejectUnknownMembers(true)); err != nil {
		var semantic *json.SemanticError
		if errors.As(err, &semantic) {
			return Envelope{}, failure("invalid_input", "$", "Required value is missing or invalid.")
		}
		return Envelope{}, failure("invalid_json", "$", "Expected one well-formed JSON object with unique member names and valid UTF-8.")
	}
	if value.SchemaVersion == "" {
		return Envelope{}, invalid("schema_version")
	}
	if value.SchemaVersion != SchemaVersion {
		return Envelope{}, failure("unsupported_version", "schema_version", "Only evidence envelope version 1 is supported.")
	}
	if !validDigest(value.SubjectDigest) {
		return Envelope{}, invalid("subject_digest")
	}
	if !validText(value.Tool) {
		return Envelope{}, invalid("tool")
	}
	if !validText(value.ToolVersion) {
		return Envelope{}, invalid("tool_version")
	}
	if !validDigest(value.ConfigurationDigest) {
		return Envelope{}, invalid("configuration_digest")
	}
	if !validText(value.Artifact.Path) {
		return Envelope{}, invalid("artifact.path")
	}
	if !validDigest(value.Artifact.Digest) {
		return Envelope{}, invalid("artifact.digest")
	}
	if value.Artifact.SizeBytes == nil || *value.Artifact.SizeBytes < 0 {
		return Envelope{}, invalid("artifact.size_bytes")
	}
	if *value.Artifact.SizeBytes > contracts.MaxBundleBytes {
		return Envelope{}, failure("limit_exceeded", "artifact.size_bytes", "Artifact exceeds the 100 MiB byte limit.")
	}
	if !validText(value.Artifact.MediaType) {
		return Envelope{}, invalid("artifact.media_type")
	}
	return value, nil
}
