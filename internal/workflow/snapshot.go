// Package workflow composes existing contracts for the single-operator snapshot profile.
package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"attestara/internal/adapters"
	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/report"
	"attestara/internal/storage"
	json "encoding/json/v2"
)

func invalid(field string) error {
	return &contracts.Error{Code: "invalid_input", Field: field, Message: "Snapshot bytes, binding or SBOM are invalid."}
}

// ReadFiles confines reads and validates the collected bytes again, avoiding a
// validation/read race. No bytes are executed or extracted from an archive.
func ReadFiles(ctx context.Context, manifest []byte, directory string) (map[string][]byte, error) {
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, invalid("files")
	}
	defer root.Close()
	input, err := bundle.ReadRoot(manifest, root)
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(input.Files))
	for _, f := range input.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		source, err := root.OpenFile(f.Path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, invalid("files")
		}
		info, e := source.Stat()
		if e != nil || !info.Mode().IsRegular() {
			source.Close()
			return nil, invalid("files")
		}
		b, e := io.ReadAll(io.LimitReader(source, *f.SizeBytes+1))
		closeErr := source.Close()
		h := sha256.Sum256(b)
		if e != nil || closeErr != nil || int64(len(b)) != *f.SizeBytes || "sha256:"+hex.EncodeToString(h[:]) != f.Digest {
			return nil, invalid("files")
		}
		files[f.Path] = b
	}
	return files, nil
}

func ValidateSBOM(input contracts.Input, files map[string][]byte) error {
	found := false
	for _, f := range input.Files {
		if f.MediaType == "application/vnd.cyclonedx+json" {
			found = true
			if _, err := adapters.DecodeCycloneDX(files[f.Path]); err != nil {
				return err
			}
		}
	}
	if !found {
		return invalid("sbom")
	}
	return nil
}

func Passport(ctx context.Context, r storage.Record) (report.Passport, error) {
	input, err := contracts.DecodeInput(ctx, r.InputJSON)
	if err != nil {
		return report.Passport{}, err
	}
	result, err := contracts.DecodeResult(ctx, r.ResultJSON)
	if err != nil {
		return report.Passport{}, err
	}
	if err := ValidateSBOM(input, r.Files); err != nil {
		return report.Passport{}, err
	}
	return report.GeneratePassport(ctx, input, result)
}

func Load(ctx context.Context, path, id string) (storage.Record, error) {
	// Reading an absent backup/store must never initialize it.
	info, err := os.Lstat(filepath.Join(path, "manifest"))
	if err != nil || !info.Mode().IsRegular() {
		return storage.Record{}, storage.ErrCorrupt
	}
	a, err := storage.Open(path)
	if err != nil {
		return storage.Record{}, err
	}
	r, err := a.Load(ctx, id)
	closeErr := a.Close()
	if err != nil {
		return r, err
	}
	return r, closeErr
}

// Copy publishes a single operation in the existing C05 format. The source is
// loaded before any destination bytes are created; no raw mutable store copy.
func Copy(ctx context.Context, source, id, destination string) (report.Passport, error) {
	r, err := Load(ctx, source, id)
	if err != nil {
		return report.Passport{}, err
	}
	passport, err := Passport(ctx, r)
	if err != nil {
		return report.Passport{}, err
	}
	parent := filepath.Dir(destination)
	temporary, err := os.MkdirTemp(parent, ".attestara-snapshot-")
	if err != nil {
		return report.Passport{}, storage.ErrUnavailable
	}
	defer os.RemoveAll(temporary)
	a, err := storage.Open(temporary)
	if err != nil {
		return report.Passport{}, err
	}
	err = a.Commit(ctx, r)
	closeErr := a.Close()
	if err != nil {
		return report.Passport{}, err
	}
	if closeErr != nil {
		return report.Passport{}, closeErr
	}
	reread, err := Load(ctx, temporary, id)
	if err != nil {
		return report.Passport{}, err
	}
	same, err := Passport(ctx, reread)
	if err != nil {
		return report.Passport{}, err
	}
	b, _ := json.Marshal(passport)
	c, _ := json.Marshal(same)
	if !bytes.Equal(b, c) {
		return report.Passport{}, storage.ErrCorrupt
	}
	if err := ctx.Err(); err != nil {
		return report.Passport{}, err
	}
	if err := publishDirectory(temporary, destination); err != nil {
		return report.Passport{}, storage.ErrUnavailable
	}
	dir, err := os.Open(parent)
	if err != nil {
		return report.Passport{}, storage.ErrUnavailable
	}
	err = dir.Sync()
	closeErr = dir.Close()
	if err != nil || closeErr != nil {
		return report.Passport{}, storage.ErrUnavailable
	}
	return passport, nil
}
