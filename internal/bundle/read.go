// Package bundle reads a versioned release-evidence manifest and its inert files.
package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"

	"attestara/internal/contracts"
)

func failure(code, field, message string) error {
	return &contracts.Error{Code: code, Field: field, Message: message}
}

// Read validates a v1 manifest and the bytes it references without executing
// them or treating a hash as proof of signer trust.
func Read(manifest []byte, files fs.FS) (contracts.Input, error) {
	if files == nil {
		return contracts.Input{}, failure("input_unavailable", "files", "File source is unavailable.")
	}
	input, err := contracts.DecodeInput(context.Background(), manifest)
	if err != nil {
		return contracts.Input{}, err
	}
	for _, file := range input.Files {
		f, err := files.Open(file.Path)
		if err != nil {
			return contracts.Input{}, failure("input_unavailable", "files.path", "A referenced file is unavailable.")
		}
		h := sha256.New()
		n, copyErr := io.Copy(h, io.LimitReader(f, *file.SizeBytes+1))
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			return contracts.Input{}, failure("input_unavailable", "files.path", "A referenced file could not be read.")
		}
		if n != *file.SizeBytes {
			return contracts.Input{}, failure("invalid_input", "files.size_bytes", "A referenced file size does not match the manifest.")
		}
		if digest := "sha256:" + hex.EncodeToString(h.Sum(nil)); digest != file.Digest {
			return contracts.Input{}, failure("invalid_input", "files.digest", "A referenced file digest does not match the manifest.")
		}
	}
	return input, nil
}
