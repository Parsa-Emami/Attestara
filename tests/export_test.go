package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/export"
)

const c23Fixtures = "fixtures/attestara/C23"

func c23Read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(c23Fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func c23Root(t *testing.T, path string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func c23Error(t *testing.T, err error, code, field string) {
	t.Helper()
	var typed *contracts.Error
	if !errors.As(err, &typed) || typed.Code != code || typed.Field != field {
		t.Fatalf("got %v; want %s/%s", err, code, field)
	}
}

func c23Rewrite(t *testing.T, archive []byte, change func(string, []byte) ([]byte, bool)) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, member := range reader.File {
		source, err := member.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(source)
		source.Close()
		if err != nil {
			t.Fatal(err)
		}
		data, keep := change(member.Name, data)
		if !keep {
			continue
		}
		dest, err := writer.Create(member.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dest.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestC23ExportAndOfflineVerification(t *testing.T) {
	manifest := c23Read(t, "manifest.json")
	archive, err := export.Build(context.Background(), manifest, c23Root(t, c23Fixtures))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "release.zip")
	if err := os.WriteFile(path, archive, 0600); err != nil {
		t.Fatal(err)
	}
	reopened, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := bundle.ReadArchive(reopened)
	if err != nil || first.Verification != "unverified" || len(first.Files) != 3 {
		t.Fatalf("offline read: %+v, %v", first, err)
	}
	second, err := bundle.ReadArchive(reopened)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("repeat offline read: %+v, %v", second, err)
	}
	modified := c23Rewrite(t, archive, func(name string, data []byte) ([]byte, bool) {
		if name == "subject.txt" {
			data[0] ^= 1
		}
		return data, true
	})
	_, err = bundle.ReadArchive(modified)
	c23Error(t, err, "invalid_input", "files.digest")
	missing := c23Rewrite(t, archive, func(name string, data []byte) ([]byte, bool) {
		return data, name != "evidence.txt"
	})
	_, err = bundle.ReadArchive(missing)
	c23Error(t, err, "input_unavailable", "files.path")
}

func TestC23RejectsBadSourcesWithoutPublishing(t *testing.T) {
	manifest := c23Read(t, "manifest.json")
	for _, tc := range []struct {
		name, path, code, field string
		data                    []byte
	}{
		{"modified", "subject.txt", "invalid_input", "files.digest", bytes.Repeat([]byte("x"), len(c23Read(t, "subject.txt")))},
		{"missing", "evidence.txt", "input_unavailable", "files.path", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"subject.txt", "evidence.txt", "policy.txt"} {
				if name == tc.path && tc.data == nil {
					continue
				}
				data := c23Read(t, name)
				if name == tc.path {
					data = tc.data
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			archive, err := export.Build(context.Background(), manifest, c23Root(t, dir))
			if archive != nil {
				t.Fatal("invalid input returned archive bytes")
			}
			c23Error(t, err, tc.code, tc.field)
		})
	}
	for _, tc := range []struct {
		name, code, field string
		data              []byte
	}{
		{"malformed", "invalid_json", "$", []byte(`{"schema_version":`)},
		{"unsupported", "unsupported_version", "schema_version", bytes.Replace(manifest, []byte(`"schema_version": "1"`), []byte(`"schema_version": "2"`), 1)},
		{"too-large", "limit_exceeded", "$", make([]byte, contracts.MaxBundleBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive, err := export.Build(context.Background(), tc.data, c23Root(t, c23Fixtures))
			if archive != nil {
				t.Fatal("invalid manifest returned archive bytes")
			}
			c23Error(t, err, tc.code, tc.field)
		})
	}
	var parsed contracts.Input
	if err := json.Unmarshal(manifest, &parsed); err != nil {
		t.Fatal(err)
	}
	parsed.Files[0].Path = "manifest.json"
	reserved, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	_, err = export.Build(context.Background(), reserved, c23Root(t, c23Fixtures))
	c23Error(t, err, "invalid_input", "files.path")
}

func TestC23SourceStaysInert(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "executed")
	script := []byte("#!/bin/sh\ntouch '" + marker + "'\n")
	for _, name := range []string{"subject.txt", "evidence.txt", "policy.txt"} {
		data := c23Read(t, name)
		if name == "subject.txt" {
			data = script
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	var manifest contracts.Input
	if err := json.Unmarshal(c23Read(t, "manifest.json"), &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(script)
	manifest.SubjectDigest = "sha256:" + hex.EncodeToString(digest[:])
	manifest.Files[0].Digest = manifest.SubjectDigest
	size := int64(len(script))
	manifest.Files[0].SizeBytes = &size
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := export.Build(context.Background(), data, c23Root(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.ReadArchive(archive); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("source was executed: %v", err)
	}
}

func TestC23ConfinesSourcePaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"evidence.txt", "policy.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), c23Read(t, name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside, err := filepath.Abs(filepath.Join(c23Fixtures, "subject.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "subject.txt")); err != nil {
		t.Fatal(err)
	}
	archive, err := export.Build(context.Background(), c23Read(t, "manifest.json"), c23Root(t, dir))
	if archive != nil {
		t.Fatal("outside-root symlink returned archive bytes")
	}
	c23Error(t, err, "input_unavailable", "files.path")
}

func TestC23CLIExportModes(t *testing.T) {
	binary := buildC20CLI(t)
	output := filepath.Join(t.TempDir(), "release.zip")
	args := []string{"export", "--input", filepath.Join(c23Fixtures, "manifest.json"), "--files", c23Fixtures, "--output", output}
	result := runC20CLI(t, binary, args...)
	if result.code != 0 || result.stdout != "" || result.stderr != "" {
		t.Fatalf("ZIP export: %+v", result)
	}
	archive, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bundle.ReadArchive(archive); err != nil {
		t.Fatal(err)
	}
	if result = runC20CLI(t, binary, args...); result.code != 3 || !strings.Contains(result.stderr, "target must not exist") {
		t.Fatalf("existing ZIP output was overwritten: %+v", result)
	}
	stillPublished, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(stillPublished, archive) {
		t.Fatalf("existing ZIP output changed: %v", err)
	}
	missingOutput := filepath.Join(t.TempDir(), "missing.zip")
	result = runC20CLI(t, binary, "export", "--input", filepath.Join(c23Fixtures, "manifest.json"), "--files", t.TempDir(), "--output", missingOutput)
	if result.code != 3 {
		t.Fatalf("missing source accepted: %+v", result)
	}
	if _, err := os.Stat(missingOutput); !os.IsNotExist(err) {
		t.Fatalf("failed export published output: %v", err)
	}
	if result = runC20CLI(t, binary, "export", "--passport", "fixtures/attestara/C20/expected.json"); result.code != 0 || c20Passport(t, result.stdout).Verification != "unverified" {
		t.Fatalf("passport export regressed: %+v", result)
	}
	if result = runC20CLI(t, binary, "export", "--input", filepath.Join(c23Fixtures, "manifest.json"), "--passport", "fixtures/attestara/C20/expected.json"); result.code != 2 {
		t.Fatalf("mixed modes accepted: %+v", result)
	}
}
