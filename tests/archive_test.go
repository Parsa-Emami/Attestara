package tests

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
	"attestara/internal/storage"
)

const c09Fixtures = "fixtures/attestara/C09"

func readC09(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(c09Fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func rejectC09(t *testing.T, data []byte, code, field string) {
	t.Helper()
	got, err := bundle.ReadArchive(data)
	var typed *contracts.Error
	if !reflect.DeepEqual(got, contracts.Input{}) || !errors.As(err, &typed) || typed.Code != code || typed.Field != field || typed.Message == "" {
		t.Fatalf("got %+v, %v; want empty result and %s/%s", got, err, code, field)
	}
}

func TestC09ReadsZIPThroughC08(t *testing.T) {
	var expected struct {
		ReleaseID    string `json:"release_id"`
		Verification string `json:"verification"`
		Files        int    `json:"files"`
	}
	if err := json.Unmarshal(readC09(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	data := readC09(t, "positive.zip")
	before := bytes.Clone(data)
	path := filepath.Join(t.TempDir(), "input.zip")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := bundle.ReadArchive(actual)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReleaseID != expected.ReleaseID || got.Verification != expected.Verification || len(got.Files) != expected.Files {
		t.Fatalf("unexpected result: %+v", got)
	}
	var expectedInput contracts.Input
	if err := json.Unmarshal(readC09(t, "manifest.json"), &expectedInput); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expectedInput) {
		t.Fatal("archive changed the typed C08 manifest")
	}
	second, err := bundle.ReadArchive(data)
	if err != nil || !reflect.DeepEqual(got, second) || !bytes.Equal(data, before) {
		t.Fatal("repeated read changed input or result", err)
	}
}

func TestC09RejectsArchiveFixtures(t *testing.T) {
	var negative struct {
		Cases []struct {
			ID      string `json:"id"`
			Archive string `json:"archive"`
			Code    string `json:"expected_code"`
			Field   string `json:"expected_field"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(readC09(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) == 0 {
		t.Fatal("missing negative fixtures")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) { rejectC09(t, readC09(t, tc.Archive), tc.Code, tc.Field) })
	}
}

func TestC09BoundsActualBytesAndChecksums(t *testing.T) {
	rejectC09(t, make([]byte, contracts.MaxBundleBytes+1), "limit_exceeded", "archive.size_bytes")
	// Bypass the claimed-size preflight with a deliberately false central-directory size.
	bomb := readC09(t, "bomb.zip")
	marker := []byte{'P', 'K', 1, 2}
	offset := bytes.LastIndex(bomb, marker)
	if offset < 0 {
		t.Fatal("missing central directory")
	}
	for i := 24; i < 28; i++ {
		bomb[offset+i] = 0
	}
	rejectC09(t, bomb, "invalid_input", "archive")
	// Tamper the checksum of an otherwise valid member, including its central header.
	data := readC09(t, "positive.zip")
	offset = bytes.LastIndex(data, marker)
	data[offset+16] ^= 1
	rejectC09(t, data, "invalid_input", "archive")
}

func TestC09SourceStaysInert(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "executed")
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	r, err := zip.NewReader(bytes.NewReader(readC09(t, "positive.zip")), int64(len(readC09(t, "positive.zip"))))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range r.File {
		src, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		dst, e := w.Create(f.Name)
		if e != nil {
			t.Fatal(e)
		}
		_, e = io.Copy(dst, src)
		src.Close()
		if e != nil {
			t.Fatal(e)
		}
	}
	directory := &zip.FileHeader{Name: "sources/"}
	directory.SetMode(os.ModeDir | 0700)
	if _, err = w.CreateHeader(directory); err != nil {
		t.Fatal(err)
	}
	h := &zip.FileHeader{Name: "sources/source.sh", Method: zip.Deflate}
	h.SetMode(0700)
	dst, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(dst, "#!/bin/sh\ntouch '"+sentinel+"'\n"); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = bundle.ReadArchive(b.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("source was executed", err)
	}
}

func TestC09InterruptedReadsPreservePriorStore(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "store")
	a, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := storageFixture(t)
	if err = a.Commit(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeout, exe, "-test.run=^TestC09ReadHelper$")
	cmd.Env = append(os.Environ(), "ATTESTARA_C09_READ_HELPER=1")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(pipe)
	if !scanner.Scan() || scanner.Text() != "reading" {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal("helper did not start reading")
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil {
		t.Fatal("helper was not interrupted")
	}
	rejectC09(t, readC09(t, "truncated.zip"), "invalid_input", "archive")
	if _, err = bundle.ReadArchive(readC09(t, "positive.zip")); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("prior manifest changed", err)
	}
	a, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	got, err := a.Load(ctx, record.OperationID)
	if err != nil || !reflect.DeepEqual(got, record) {
		t.Fatal("prior durable record changed", err)
	}
}

func TestC09ReadHelper(t *testing.T) {
	if os.Getenv("ATTESTARA_C09_READ_HELPER") != "1" {
		return
	}
	data := readC09(t, "positive.zip")
	if _, err := bundle.ReadArchive(data); err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("reading\n")
	for {
		if _, err := bundle.ReadArchive(data); err != nil {
			t.Fatal(err)
		}
	}
}
