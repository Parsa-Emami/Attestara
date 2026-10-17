package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"attestara/internal/contracts"
	"attestara/internal/storage"
)

func storageFixture(t *testing.T) storage.Record {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("fixtures/attestara/C04", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var positive struct {
		SchemaVersion int    `json:"schema_version"`
		OperationID   string `json:"operation_id"`
		InputFixture  string `json:"input_fixture"`
		ResultFixture string `json:"result_fixture"`
	}
	b, err := os.ReadFile("fixtures/attestara/C05/positive.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &positive); err != nil || positive.SchemaVersion != 1 {
		t.Fatal("C05 fixture version")
	}
	r := storage.Record{OperationID: positive.OperationID, InputJSON: read(positive.InputFixture), ResultJSON: read(positive.ResultFixture), Files: map[string][]byte{}}
	v, err := contracts.DecodeInput(context.Background(), r.InputJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range v.Files {
		r.Files[f.Path] = read(f.Path)
	}
	return r
}

func TestC05CommitRestartReplayAndConflict(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "store")
	a, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]any
	b, err := os.ReadFile("fixtures/attestara/C05/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &expected); err != nil {
		t.Fatal(err)
	}
	if expected["conflict"] != storage.ErrConflict.Error() || expected["corrupt"] != storage.ErrCorrupt.Error() || expected["writer_busy"] != storage.ErrLocked.Error() {
		t.Fatal("fixture error semantics drifted")
	}
	r := storageFixture(t)
	if err := a.Commit(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	a, err = storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	got, err := a.Load(ctx, r.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.InputJSON, r.InputJSON) || !bytes.Equal(got.ResultJSON, r.ResultJSON) {
		t.Fatal("exact JSON bytes changed")
	}
	for name, want := range r.Files {
		if !bytes.Equal(got.Files[name], want) {
			t.Fatal(name)
		}
	}
	before, _ := os.ReadFile(filepath.Join(dir, "manifest"))
	if err := a.Commit(ctx, r); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "manifest"))
	if !bytes.Equal(before, after) {
		t.Fatal("replay wrote another transaction")
	}
	r.ResultJSON = append([]byte(" \n"), r.ResultJSON...)
	if err := a.Commit(ctx, r); !errors.Is(err, storage.ErrConflict) {
		t.Fatal(err)
	}
	if _, err := a.Load(ctx, "missing"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestC05SingleWriterAndCorruptReferences(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	a, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := storage.Open(dir); !errors.Is(err, storage.ErrLocked) {
		if other != nil {
			other.Close()
		}
		t.Fatal(err)
	}
	r := storageFixture(t)
	if err := a.Commit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	a.Close()
	input, _ := contracts.DecodeInput(context.Background(), r.InputJSON)
	if err := os.Remove(filepath.Join(dir, "objects", input.SubjectDigest[7:])); err != nil {
		t.Fatal(err)
	}
	if b, err := storage.Open(dir); !errors.Is(err, storage.ErrCorrupt) {
		if b != nil {
			b.Close()
		}
		t.Fatal(err)
	}
}

func TestC05RejectsInvalidBytesPathsAndInputBinding(t *testing.T) {
	var negative struct {
		InvalidCases []string `json:"invalid_cases"`
	}
	b, err := os.ReadFile("fixtures/attestara/C05/negative.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &negative); err != nil || len(negative.InvalidCases) == 0 {
		t.Fatal("missing negative cases")
	}
	for _, kind := range negative.InvalidCases {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			a, err := storage.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			r := storageFixture(t)
			ctx := context.Background()
			switch kind {
			case "digest":
				r.Files["subject.txt"][0] ^= 1
			case "size":
				r.Files["subject.txt"] = nil
			case "path":
				r.InputJSON = bytes.ReplaceAll(r.InputJSON, []byte("subject.txt"), []byte("../escape"))
			case "binding":
				r.InputJSON = append(r.InputJSON, '\n')
			case "extra":
				r.Files["unexpected"] = []byte("inert")
			case "cancel":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			if err := a.Commit(ctx, r); err == nil {
				t.Fatal("invalid transaction accepted")
			}
			b, _ := os.ReadFile(filepath.Join(dir, "manifest"))
			if string(b) != "{\"schema_version\":1}\n" {
				t.Fatal("failed transaction changed manifest")
			}
		})
	}
}
