// Package bundle reads a versioned release-evidence manifest and its inert files.
package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"syscall"

	"attestara/internal/contracts"
)

func failure(code, field, message string) error {
	return &contracts.Error{Code: code, Field: field, Message: message}
}

// readFS validates a v1 manifest against a safe, immutable virtual filesystem.
func readFS(manifest []byte, files fs.FS) (contracts.Input, error) {
	if files == nil {
		return contracts.Input{}, failure("input_unavailable", "files", "File source is unavailable.")
	}
	return read(manifest, files.Open)
}

// ReadRoot reads host files through a confined root. Symlinks may resolve only
// within that root; blocking or non-regular filesystem entries are rejected.
func ReadRoot(manifest []byte, root *os.Root) (contracts.Input, error) {
	if root == nil {
		return contracts.Input{}, failure("input_unavailable", "files", "File source is unavailable.")
	}
	return read(manifest, func(path string) (fs.File, error) {
		return root.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	})
}

func read(manifest []byte, open func(string) (fs.File, error)) (contracts.Input, error) {
	input, err := contracts.DecodeInput(context.Background(), manifest)
	if err != nil {
		return contracts.Input{}, err
	}
	for _, file := range input.Files {
		f, err := open(file.Path)
		if err != nil {
			return contracts.Input{}, failure("input_unavailable", "files.path", "A referenced file is unavailable.")
		}
		info, statErr := f.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			_ = f.Close()
			return contracts.Input{}, failure("input_unavailable", "files.path", "A referenced file is not a regular file.")
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
