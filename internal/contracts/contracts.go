// Package contracts validates the v1 release-evidence boundary. It does not
// execute policy, read manifest paths, establish ownership, or verify signatures.
package contracts

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxBundleBytes     int64 = 100 * 1024 * 1024
	MaxFiles                 = 2000
	ValidationDeadline       = 10 * time.Second
)

// Error contains a stable code and safe field/message, never raw input bytes.
type Error struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Field + ": " + e.Message }

// File is manifest metadata; a later storage/subject adapter checks actual bytes.
type File struct {
	Path      string `json:"path"`
	Digest    string `json:"digest"`
	SizeBytes *int64 `json:"size_bytes"`
	MediaType string `json:"media_type"`
}
type Input struct {
	SchemaVersion  string `json:"schema_version"`
	ReleaseID      string `json:"release_id"`
	Owner          string `json:"owner"`
	SubjectDigest  string `json:"subject_digest"`
	SourceRevision string `json:"source_revision"`
	Tool           string `json:"tool"`
	ToolVersion    string `json:"tool_version"`
	EvidenceDigest string `json:"evidence_digest"`
	PolicyDigest   string `json:"policy_digest"`
	Verification   string `json:"verification"`
	Files          []File `json:"files"`
}
type Outcome struct {
	SubjectDigest  string `json:"subject_digest"`
	EvidenceDigest string `json:"evidence_digest"`
	PolicyDigest   string `json:"policy_digest"`
	PolicyDecision string `json:"policy_decision"`
	Verification   string `json:"verification"`
}
type Result struct {
	SchemaVersion string   `json:"schema_version"`
	Status        string   `json:"status"`
	InputDigest   string   `json:"input_digest"`
	EngineVersion string   `json:"engine_version"`
	Result        *Outcome `json:"result,omitempty"`
	Warnings      []string `json:"warnings"`
	Error         *Error   `json:"error,omitempty"`
}

var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var pathPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._-]*(/[A-Za-z0-9_-][A-Za-z0-9._-]*)*$`)
var errorCodes = map[string]bool{"invalid_json": true, "invalid_input": true, "unsupported_version": true, "limit_exceeded": true, "cancelled": true, "deadline_exceeded": true, "input_unavailable": true, "dependency_unavailable": true}

func failure(code, field, message string) error {
	if field == "" {
		field = "$"
	}
	return &Error{code, field, message}
}
func invalid(field string) error {
	return failure("invalid_input", field, "Required value is missing or invalid.")
}
func text(s string) bool { return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 4096 }
func interrupted(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return failure("deadline_exceeded", "", "Validation deadline exceeded.")
	}
	if ctx.Err() != nil {
		return failure("cancelled", "", "Validation cancelled.")
	}
	return nil
}
func decode(ctx context.Context, data []byte, out any) error {
	if err := interrupted(ctx); err != nil {
		return err
	}
	if int64(len(data)) > MaxBundleBytes {
		return failure("limit_exceeded", "", "JSON exceeds the 100 MiB byte limit.")
	}
	// Go 1.27 JSON v2 rejects duplicate names, invalid UTF-8 and case folding.
	// https://pkg.go.dev/encoding/json/v2#Unmarshal
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		return decodeError(err)
	}
	if fields == nil {
		return invalid("")
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return invalid("")
		}
	}
	// Count before allocating the typed slice; a hostile null array must not
	// allocate millions of File values before its 2000-file limit is checked.
	if _, input := out.(*Input); input && len(fields["files"]) != 0 {
		d := jsontext.NewDecoder(bytes.NewReader(fields["files"]))
		if token, err := d.ReadToken(); err != nil || token.Kind() != '[' {
			return invalid("files")
		}
		for count := 0; d.PeekKind() != ']'; count++ {
			if err := interrupted(ctx); err != nil {
				return err
			}
			if count >= MaxFiles {
				return failure("limit_exceeded", "files", "Bundle exceeds 2000 files.")
			}
			if err := d.SkipValue(); err != nil {
				return decodeError(err)
			}
		}
	}
	if err := json.Unmarshal(data, out, json.RejectUnknownMembers(true)); err != nil {
		return decodeError(err)
	}
	return interrupted(ctx)
}
func decodeError(err error) error {
	var semantic *json.SemanticError
	if errors.As(err, &semantic) {
		return invalid("")
	}
	return failure("invalid_json", "", "Expected one well-formed JSON object with unique member names and valid UTF-8.")
}
func version(v string) error {
	if v == "" {
		return invalid("schema_version")
	}
	if v != "1" {
		return failure("unsupported_version", "schema_version", "Only executable contract version 1 is supported; draft 0 requires explicit re-authoring.")
	}
	return nil
}

// DecodeInput validates immutable JSON bytes under a cooperative 10 second cap.
// Cancellation is checked before/after bounded decoding and during file traversal.
func DecodeInput(parent context.Context, data []byte) (Input, error) {
	ctx, cancel := context.WithTimeout(parent, ValidationDeadline)
	defer cancel()
	var v Input
	if err := decode(ctx, data, &v); err != nil {
		return Input{}, err
	}
	if err := version(v.SchemaVersion); err != nil {
		return Input{}, err
	}
	for _, f := range []struct{ name, value string }{{"release_id", v.ReleaseID}, {"owner", v.Owner}, {"source_revision", v.SourceRevision}, {"tool", v.Tool}, {"tool_version", v.ToolVersion}} {
		if !text(f.value) {
			return Input{}, invalid(f.name)
		}
	}
	for _, f := range []struct{ name, value string }{{"subject_digest", v.SubjectDigest}, {"evidence_digest", v.EvidenceDigest}, {"policy_digest", v.PolicyDigest}} {
		if !digestPattern.MatchString(f.value) {
			return Input{}, invalid(f.name)
		}
	}
	if v.Verification != "unverified" {
		return Input{}, invalid("verification")
	}
	if len(v.Files) == 0 {
		return Input{}, invalid("files")
	}
	if len(v.Files) > MaxFiles {
		return Input{}, failure("limit_exceeded", "files", "Bundle exceeds 2000 files.")
	}
	paths := make(map[string]bool, len(v.Files))
	digests := make(map[string]bool, len(v.Files))
	var total int64
	for _, f := range v.Files {
		if err := interrupted(ctx); err != nil {
			return Input{}, err
		}
		if !text(f.Path) || !pathPattern.MatchString(f.Path) || paths[f.Path] {
			return Input{}, invalid("files.path")
		}
		if !digestPattern.MatchString(f.Digest) {
			return Input{}, invalid("files.digest")
		}
		if f.SizeBytes == nil || *f.SizeBytes < 0 {
			return Input{}, invalid("files.size_bytes")
		}
		if *f.SizeBytes > MaxBundleBytes-total {
			return Input{}, failure("limit_exceeded", "files.size_bytes", "Bundle exceeds 100 MiB of declared file bytes.")
		}
		if !text(f.MediaType) {
			return Input{}, invalid("files.media_type")
		}
		total += *f.SizeBytes
		paths[f.Path] = true
		digests[f.Digest] = true
	}
	if !digests[v.SubjectDigest] || !digests[v.EvidenceDigest] || !digests[v.PolicyDigest] {
		return Input{}, invalid("files")
	}
	if err := interrupted(ctx); err != nil {
		return Input{}, err
	}
	return v, nil
}

// DecodeResult validates a serialized domain result, without treating its content
// as proof that a policy, signature or provider actually ran. Trusted is withheld
// until cryptographic trust has an implemented contract and verified evidence.
func DecodeResult(parent context.Context, data []byte) (Result, error) {
	ctx, cancel := context.WithTimeout(parent, ValidationDeadline)
	defer cancel()
	var v Result
	if err := decode(ctx, data, &v); err != nil {
		return Result{}, err
	}
	if err := version(v.SchemaVersion); err != nil {
		return Result{}, err
	}
	if !digestPattern.MatchString(v.InputDigest) {
		return Result{}, invalid("input_digest")
	}
	if !text(v.EngineVersion) {
		return Result{}, invalid("engine_version")
	}
	if v.Warnings == nil {
		return Result{}, invalid("warnings")
	}
	for _, s := range v.Warnings {
		if !text(s) {
			return Result{}, invalid("warnings")
		}
	}
	switch v.Status {
	case "failed":
		if v.Result != nil || v.Error == nil || !errorCodes[v.Error.Code] || !text(v.Error.Message) || !text(v.Error.Field) {
			return Result{}, invalid("error")
		}
	case "complete", "partial", "unverified":
		if v.Error != nil || v.Result == nil {
			return Result{}, invalid("result")
		}
		o := v.Result
		if !digestPattern.MatchString(o.SubjectDigest) || !digestPattern.MatchString(o.EvidenceDigest) || !digestPattern.MatchString(o.PolicyDigest) {
			return Result{}, invalid("result")
		}
		if o.Verification != "unverified" && o.Verification != "rejected" {
			return Result{}, invalid("result.verification")
		}
		if o.PolicyDecision != "allow" && o.PolicyDecision != "deny" && o.PolicyDecision != "unknown" {
			return Result{}, invalid("result.policy_decision")
		}
		if v.Status == "complete" && o.PolicyDecision == "unknown" {
			return Result{}, invalid("status")
		}
		if v.Status == "unverified" && o.Verification != "unverified" {
			return Result{}, invalid("status")
		}
	default:
		return Result{}, invalid("status")
	}
	if err := interrupted(ctx); err != nil {
		return Result{}, err
	}
	return v, nil
}
