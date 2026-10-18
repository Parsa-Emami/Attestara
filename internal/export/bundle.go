// Package export packages inert release inputs for the existing offline bundle reader.
package export

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"syscall"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
)

func failure(code, field, message string) error {
	return &contracts.Error{Code: code, Field: field, Message: message}
}

// Build returns a C09-compatible ZIP with the original v1 manifest and every
// referenced file. Source files are confined to root and are never executed.
func Build(ctx context.Context, manifest []byte, root *os.Root) ([]byte, error) {
	if root == nil {
		return nil, failure("input_unavailable", "files", "File source is unavailable.")
	}
	input, err := contracts.DecodeInput(ctx, manifest)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	add := func(name string, data []byte) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		dst, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = dst.Write(data)
		return err
	}
	if err := add("manifest.json", manifest); err != nil {
		return nil, failure("input_unavailable", "archive", "Archive could not be written.")
	}
	for _, file := range input.Files {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, failure("deadline_exceeded", "$", "Export deadline exceeded.")
			}
			return nil, failure("cancelled", "$", "Export cancelled.")
		}
		if file.Path == "manifest.json" {
			return nil, failure("invalid_input", "files.path", "A file cannot replace the archive manifest.")
		}
		source, err := root.OpenFile(file.Path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, failure("input_unavailable", "files.path", "A referenced file is unavailable.")
		}
		info, statErr := source.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			_ = source.Close()
			return nil, failure("input_unavailable", "files.path", "A referenced file is not a regular file.")
		}
		header := &zip.FileHeader{Name: file.Path, Method: zip.Deflate}
		header.SetMode(0600)
		dst, err := writer.CreateHeader(header)
		if err != nil {
			_ = source.Close()
			return nil, failure("input_unavailable", "archive", "Archive could not be written.")
		}
		hash := sha256.New()
		n, copyErr := io.CopyN(io.MultiWriter(dst, hash), source, *file.SizeBytes+1)
		closeErr := source.Close()
		if copyErr != nil && !errors.Is(copyErr, io.EOF) || closeErr != nil {
			return nil, failure("input_unavailable", "files.path", "A referenced file could not be read.")
		}
		if n != *file.SizeBytes {
			return nil, failure("invalid_input", "files.size_bytes", "A referenced file size does not match the manifest.")
		}
		if "sha256:"+hex.EncodeToString(hash.Sum(nil)) != file.Digest {
			return nil, failure("invalid_input", "files.digest", "A referenced file digest does not match the manifest.")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, failure("input_unavailable", "archive", "Archive could not be completed.")
	}
	if int64(output.Len()) > contracts.MaxBundleBytes {
		return nil, failure("limit_exceeded", "archive.size_bytes", "ZIP exceeds the 100 MiB input limit.")
	}
	if _, err := bundle.ReadArchive(output.Bytes()); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
