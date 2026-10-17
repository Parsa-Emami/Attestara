package bundle

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"strings"

	"attestara/internal/contracts"
)

// ReadArchive validates an inert ZIP with a root manifest.json through Read.
// The caller must bound input before allocating data and must not mutate it
// during validation. No member is extracted, executed, or persisted.
func ReadArchive(data []byte) (contracts.Input, error) {
	if int64(len(data)) > contracts.MaxBundleBytes {
		return contracts.Input{}, failure("limit_exceeded", "archive.size_bytes", "ZIP exceeds the 100 MiB input limit.")
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return contracts.Input{}, failure("invalid_input", "archive", "Expected a well-formed ZIP archive.")
	}
	if len(r.File) > contracts.MaxFiles+1 {
		return contracts.Input{}, failure("limit_exceeded", "archive.entries", "ZIP exceeds 2000 data entries plus its manifest.")
	}
	seen := make(map[string]bool, len(r.File))
	var manifestFile *zip.File
	var total uint64
	for _, f := range r.File {
		name := f.Name
		if f.Mode().IsDir() {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || seen[name] || (f.Mode().Type() != 0 && f.Mode().Type() != fs.ModeDir) {
			return contracts.Input{}, failure("invalid_input", "archive.path", "ZIP paths must be unique local regular files or directories.")
		}
		seen[name] = true
		if f.Mode().IsDir() && f.UncompressedSize64 != 0 {
			return contracts.Input{}, failure("invalid_input", "archive.path", "ZIP directories must have no content.")
		}
		if name == "manifest.json" && !f.Mode().IsDir() {
			manifestFile = f
			continue
		}
		if f.UncompressedSize64 > uint64(contracts.MaxBundleBytes)-total {
			return contracts.Input{}, failure("limit_exceeded", "archive.size_bytes", "ZIP exceeds 100 MiB of expanded data.")
		}
		total += f.UncompressedSize64
	}
	if manifestFile == nil {
		return contracts.Input{}, failure("input_unavailable", "manifest.json", "ZIP root manifest.json is unavailable.")
	}
	if manifestFile.UncompressedSize64 > uint64(contracts.MaxBundleBytes) {
		return contracts.Input{}, failure("limit_exceeded", "archive.size_bytes", "ZIP manifest exceeds 100 MiB.")
	}
	var manifest []byte
	var actual int64
	// Read every member, including unreferenced ones: ZIP sizes are untrusted,
	// and checksums are only checked when the decompressor reaches EOF.
	for _, f := range r.File {
		src, e := f.Open()
		if e != nil {
			return contracts.Input{}, failure("invalid_input", "archive", "A ZIP member could not be opened.")
		}
		limit := contracts.MaxBundleBytes - actual
		if f == manifestFile {
			limit = contracts.MaxBundleBytes
		}
		var n int64
		if f == manifestFile {
			manifest, e = io.ReadAll(io.LimitReader(src, limit+1))
			n = int64(len(manifest))
		} else {
			n, e = io.Copy(io.Discard, io.LimitReader(src, limit+1))
		}
		closeErr := src.Close()
		if n > limit {
			return contracts.Input{}, failure("limit_exceeded", "archive.size_bytes", "ZIP expanded bytes exceed the 100 MiB limit.")
		}
		if e != nil || closeErr != nil || uint64(n) != f.UncompressedSize64 {
			return contracts.Input{}, failure("invalid_input", "archive", "A ZIP member is truncated or corrupt.")
		}
		if f != manifestFile {
			actual += n
		}
	}
	return Read(manifest, r)
}
