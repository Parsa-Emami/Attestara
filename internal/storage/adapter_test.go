package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"attestara/internal/contracts"
)

type negativeFixture struct {
	IO      []string `json:"io_failure_phases"`
	Crash   []string `json:"crash_phases"`
	Corrupt []string `json:"corrupt_cases"`
}

func negative(t *testing.T) negativeFixture {
	t.Helper()
	b, err := os.ReadFile("../../tests/fixtures/attestara/C05/negative.json")
	if err != nil {
		t.Fatal(err)
	}
	var n negativeFixture
	if err := json.Unmarshal(b, &n); err != nil {
		t.Fatal(err)
	}
	return n
}
func fixture(t *testing.T, id string, changed bool) Record {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile("../../tests/fixtures/attestara/C04/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	r := Record{OperationID: id, InputJSON: read("input.valid.json"), ResultJSON: read("result.valid.json"), Files: map[string][]byte{}}
	input, err := contracts.DecodeInput(context.Background(), r.InputJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range input.Files {
		r.Files[f.Path] = read(f.Path)
	}
	if changed {
		r.Files["subject.txt"] = append(r.Files["subject.txt"], []byte(" changed")...)
		updateSubject(t, &r)
	}
	return r
}
func updateSubject(t *testing.T, r *Record) {
	t.Helper()
	v, err := contracts.DecodeInput(context.Background(), r.InputJSON)
	if err != nil {
		t.Fatal(err)
	}
	v.SubjectDigest = digest(r.Files["subject.txt"])
	for i := range v.Files {
		if v.Files[i].Path == "subject.txt" {
			n := int64(len(r.Files["subject.txt"]))
			v.Files[i].SizeBytes = &n
			v.Files[i].Digest = v.SubjectDigest
		}
	}
	r.InputJSON, err = json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	result, err := contracts.DecodeResult(context.Background(), r.ResultJSON)
	if err != nil {
		t.Fatal(err)
	}
	result.InputDigest = digest(r.InputJSON)
	result.Result.SubjectDigest = v.SubjectDigest
	r.ResultJSON, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
}
func open(t *testing.T, dir string) *Adapter {
	t.Helper()
	a, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func accepted(t *testing.T, a *Adapter, r Record) {
	t.Helper()
	if err := a.Commit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
}

func TestC05QuotaBoundaries(t *testing.T) {
	r := fixture(t, "quota", false)
	input, err := contracts.DecodeInput(context.Background(), r.InputJSON)
	if err != nil {
		t.Fatal(err)
	}
	var historyBytes int64
	objects := make(map[string]int64, len(input.Files))
	var objectBytes int64
	for _, file := range input.Files {
		historyBytes += *file.SizeBytes
		if _, exists := objects[file.Digest]; !exists {
			objects[file.Digest] = *file.SizeBytes
			objectBytes += *file.SizeBytes
		}
	}
	for _, tc := range []struct {
		name  string
		usage storeUsage
		want  error
	}{
		{"within limits", storeUsage{rewrittenBytes: int64(len(header)), objects: map[string]int64{}}, nil},
		{"operation count", storeUsage{operations: maxOperations, rewrittenBytes: int64(len(header)), objects: map[string]int64{}}, ErrQuota},
		{"artifact references", storeUsage{artifactRefs: maxArtifactRefs - len(input.Files) + 1, rewrittenBytes: int64(len(header)), objects: map[string]int64{}}, ErrQuota},
		{"history bytes", storeUsage{historyBytes: maxHistoryBytes - historyBytes + 1, rewrittenBytes: int64(len(header)), objects: map[string]int64{}}, ErrQuota},
		{"object bytes", storeUsage{objectBytes: maxObjectBytes - objectBytes + 1, rewrittenBytes: int64(len(header)), objects: map[string]int64{}}, ErrQuota},
		{"manifest bytes", storeUsage{rewrittenBytes: maxManifestBytes, objects: map[string]int64{}}, ErrQuota},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkQuota(tc.usage, input, 1)
			if !errors.Is(err, tc.want) {
				t.Fatalf("checkQuota() = %v; want %v", err, tc.want)
			}
		})
	}
	if err := checkQuota(storeUsage{operations: maxOperations - 1, artifactRefs: maxArtifactRefs - len(input.Files), historyBytes: maxHistoryBytes - historyBytes, objectBytes: maxObjectBytes - objectBytes, rewrittenBytes: maxManifestBytes - 1, objects: map[string]int64{}}, input, 1); err != nil {
		t.Fatalf("exact quota boundary rejected: %v", err)
	}
}

func TestC05QuotaCommitPreservesFullSnapshot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	a := open(t, dir)
	seed := fixture(t, "quota-0000", false)
	accepted(t, a, seed)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	manifest := bytes.NewBufferString(header)
	for i := 0; i < maxOperations; i++ {
		e := entry{OperationID: fmt.Sprintf("quota-%04d", i), Input: seed.InputJSON, Result: seed.ResultJSON}
		e.Checksum = checksum(e)
		line, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Write(line)
		manifest.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest"), manifest.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	a = open(t, dir)
	before, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Commit(context.Background(), fixture(t, "quota-overflow", false)); !errors.Is(err, ErrQuota) {
		t.Fatalf("Commit() = %v; want ErrQuota", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("quota rejection changed the accepted manifest")
	}
	seed.OperationID = "quota-0000"
	assertLoad(t, a, seed, true)
	if _, err := os.Stat(filepath.Join(dir, "staging", "blob")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("quota rejection wrote a staged blob: %v", err)
	}
}
func assertLoad(t *testing.T, a *Adapter, r Record, want bool) {
	t.Helper()
	got, err := a.Load(context.Background(), r.OperationID)
	if !want {
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected absent operation: %v", err)
		}
		return
	}
	if err != nil || !bytes.Equal(got.InputJSON, r.InputJSON) || !bytes.Equal(got.ResultJSON, r.ResultJSON) {
		t.Fatalf("lost accepted JSON: %v", err)
	}
	for p, b := range r.Files {
		if !bytes.Equal(b, got.Files[p]) {
			t.Fatal("lost artifact", p)
		}
	}
}
func published(phase string) bool { return phase == "directory.sync" || phase == "committed" }

func TestC05InjectedDiskFullRecovery(t *testing.T) {
	for _, phase := range negative(t).IO {
		t.Run(phase, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			a := open(t, dir)
			base := fixture(t, "base", false)
			next := fixture(t, "next", true)
			accepted(t, a, base)
			fired := false
			a.checkpoint = func(p string) error {
				if p == phase {
					fired = true
					if phase == "object.write" || phase == "manifest.write" {
						name := "staging/blob"
						if phase == "manifest.write" {
							name = "manifest.next"
						}
						f, err := a.root.OpenFile(name, os.O_WRONLY, 0)
						if err != nil {
							t.Fatal(err)
						}
						if _, err = f.Write([]byte("partial")); err != nil {
							t.Fatal(err)
						}
						f.Close()
					}
					return syscall.ENOSPC
				}
				return nil
			}
			if err := a.Commit(context.Background(), next); !errors.Is(err, syscall.ENOSPC) || !errors.Is(err, ErrUnavailable) || !fired {
				t.Fatal("fault not exercised", err)
			}
			if _, err := a.Load(context.Background(), base.OperationID); !errors.Is(err, ErrRecovery) {
				t.Fatal("uncertain writer reused", err)
			}
			a.Close()
			a = open(t, dir)
			assertLoad(t, a, base, true)
			assertLoad(t, a, next, published(phase))
			beforeObjects, _ := os.ReadDir(filepath.Join(dir, "objects"))
			wantObjects := 3
			if published(phase) {
				wantObjects = 4
			}
			if len(beforeObjects) != wantObjects {
				t.Fatal("recovery retained orphan objects")
			}
			if _, err := os.Stat(filepath.Join(dir, "manifest.next")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("temporary manifest survived recovery")
			}
			accepted(t, a, next)
			before, _ := os.ReadFile(filepath.Join(dir, "manifest"))
			accepted(t, a, next)
			after, _ := os.ReadFile(filepath.Join(dir, "manifest"))
			if !bytes.Equal(before, after) {
				t.Fatal("replayed transaction")
			}
			entries, _ := os.ReadDir(filepath.Join(dir, "staging"))
			if len(entries) != 0 {
				t.Fatal("orphan staging remains")
			}
		})
	}
}

func TestC05CrashHelper(t *testing.T) {
	dir := os.Getenv("ATTESTARA_C05_TEST_DIR")
	if dir == "" {
		return
	}
	if os.Getenv("ATTESTARA_C05_TEST_PHASE") == "lock" {
		a, err := Open(dir)
		if a != nil {
			a.Close()
		}
		if !errors.Is(err, ErrLocked) {
			t.Fatal(err)
		}
		return
	}
	a := open(t, dir)
	phase := os.Getenv("ATTESTARA_C05_TEST_PHASE")
	a.checkpoint = func(p string) error {
		if p == phase {
			syscall.Kill(os.Getpid(), syscall.SIGKILL)
		}
		return nil
	}
	if err := a.Commit(context.Background(), fixture(t, "next", true)); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash checkpoint not reached")
}
func child(t *testing.T, dir, phase string) error {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestC05CrashHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "ATTESTARA_C05_TEST_DIR="+dir, "ATTESTARA_C05_TEST_PHASE="+phase)
	b, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("helper exceeded deadline", ctx.Err())
	}
	if err != nil && !strings.Contains(err.Error(), "killed") {
		t.Fatalf("helper failed: %v: %s", err, b)
	}
	return err
}
func TestC05RealProcessCrashRecoveryAndWriterLock(t *testing.T) {
	for _, phase := range negative(t).Crash {
		t.Run(phase, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			a := open(t, dir)
			base := fixture(t, "base", false)
			accepted(t, a, base)
			if err := child(t, dir, "lock"); err != nil {
				t.Fatal(err)
			}
			a.Close()
			if err := child(t, dir, phase); err == nil {
				t.Fatal("process did not crash")
			}
			a = open(t, dir)
			next := fixture(t, "next", true)
			assertLoad(t, a, base, true)
			assertLoad(t, a, next, published(phase))
			beforeObjects, _ := os.ReadDir(filepath.Join(dir, "objects"))
			wantObjects := 3
			if published(phase) {
				wantObjects = 4
			}
			if len(beforeObjects) != wantObjects {
				t.Fatal("recovery retained orphan objects")
			}
			if _, err := os.Stat(filepath.Join(dir, "manifest.next")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("temporary manifest survived recovery")
			}
			accepted(t, a, next)
			objects, _ := os.ReadDir(filepath.Join(dir, "objects"))
			if len(objects) != 4 {
				t.Fatalf("orphan/dedup mismatch: %d", len(objects))
			}
		})
	}
}
func TestC05CorruptionFailsClosedAndPreservesManifest(t *testing.T) {
	for _, kind := range negative(t).Corrupt {
		t.Run(kind, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			a := open(t, dir)
			r := fixture(t, "base", false)
			accepted(t, a, r)
			a.Close()
			manifest := filepath.Join(dir, "manifest")
			b, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			v, _ := contracts.DecodeInput(context.Background(), r.InputJSON)
			obj := filepath.Join(dir, "objects", v.SubjectDigest[7:])
			switch kind {
			case "version":
				b = bytes.Replace(b, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1)
			case "truncated":
				b = b[:len(b)-1]
			case "checksum":
				b = bytes.Replace(b, []byte(`"checksum":"sha256:`), []byte(`"checksum":"sha256:0`), 1)
			case "duplicate":
				b = append(b, b[len(header):]...)
			case "unknown-field":
				b = bytes.Replace(b, []byte(`"operation_id":`), []byte(`"unknown":true,"operation_id":`), 1)
			case "missing-manifest":
				os.Remove(manifest)
			case "missing-object":
				os.Remove(obj)
			case "changed-object":
				os.WriteFile(obj, []byte("corrupt"), 0600)
			case "symlink":
				os.Remove(obj)
				outside := filepath.Join(t.TempDir(), "outside")
				os.WriteFile(outside, r.Files["subject.txt"], 0600)
				os.Symlink(outside, obj)
			case "hardlink":
				outside := filepath.Join(t.TempDir(), "outside")
				if err := os.Link(obj, outside); err != nil {
					t.Fatal(err)
				}
			case "forged-orphan":
				os.WriteFile(filepath.Join(dir, "objects", strings.Repeat("0", 64)), []byte("foreign-content"), 0600)
			case "unknown-object":
				os.WriteFile(filepath.Join(dir, "objects", "not-owned"), []byte("preserve"), 0600)
			default:
				t.Fatal("unknown fixture case", kind)
			}
			if kind == "version" || kind == "truncated" || kind == "checksum" || kind == "duplicate" || kind == "unknown-field" {
				if err := os.WriteFile(manifest, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, readErr := os.ReadFile(manifest)
			a, err = Open(dir)
			if a != nil {
				a.Close()
			}
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("accepted corrupt %s: %v", kind, err)
			}
			after, afterErr := os.ReadFile(manifest)
			if !bytes.Equal(before, after) || errors.Is(readErr, os.ErrNotExist) != errors.Is(afterErr, os.ErrNotExist) {
				t.Fatal("corrupt state was silently repaired")
			}
		})
	}
}
func TestC05CancellationAndConcurrentAdmission(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	a := open(t, dir)
	r := fixture(t, "base", false)
	entered, release := make(chan struct{}), make(chan struct{})
	a.checkpoint = func(p string) error {
		if p == "objects.ready" {
			close(entered)
			<-release
		}
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- a.Commit(context.Background(), r) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not reach checkpoint")
	}
	if err := a.Commit(context.Background(), r); !errors.Is(err, ErrLocked) {
		t.Fatal(err)
	}
	if _, err := a.Load(context.Background(), r.OperationID); !errors.Is(err, ErrLocked) {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not finish")
	}
	for _, phase := range []string{"objects.ready", "directory.sync"} {
		t.Run(phase, func(t *testing.T) {
			a := open(t, filepath.Join(t.TempDir(), "store"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			a.checkpoint = func(p string) error {
				if p == phase {
					cancel()
				}
				return nil
			}
			err := a.Commit(ctx, r)
			if phase == "directory.sync" && err != nil {
				t.Fatal("published cancellation lost acknowledgement", err)
			}
			if phase == "objects.ready" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}
func TestC05SourceBytesStayInertAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	a := open(t, dir)
	r := fixture(t, "inert", false)
	marker := filepath.Join(t.TempDir(), "executed")
	r.Files["subject.txt"] = []byte("#!/bin/sh\ntouch " + marker + "\n")
	updateSubject(t, &r)
	accepted(t, a, r)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source executed")
	}
	a.Close()
	a = open(t, dir)
	assertLoad(t, a, r, true)
	f, err := os.Stat(filepath.Join(dir, "objects", digest(r.Files["subject.txt"])[7:]))
	if err != nil || f.Mode().Perm() != 0600 {
		t.Fatal("artifact permissions", err)
	}
}
func TestC05PrivateRootAndUnknownStaging(t *testing.T) {
	parent := t.TempDir()
	outside := filepath.Join(parent, "outside")
	os.Mkdir(outside, 0700)
	link := filepath.Join(parent, "link")
	os.Symlink(outside, link)
	if a, err := Open(link + string(os.PathSeparator)); !errors.Is(err, ErrCorrupt) {
		if a != nil {
			a.Close()
		}
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "store")
	a := open(t, dir)
	a.Close()
	unknown := filepath.Join(dir, "staging", "foreign")
	os.WriteFile(unknown, []byte("preserve"), 0600)
	if a, err := Open(dir); !errors.Is(err, ErrCorrupt) {
		if a != nil {
			a.Close()
		}
		t.Fatal(err)
	}
	if b, err := os.ReadFile(unknown); err != nil || string(b) != "preserve" {
		t.Fatal("unknown file deleted")
	}
}

func TestC05InterruptedInitialization(t *testing.T) {
	for _, partial := range []string{"", header[:5], header} {
		t.Run(fmt.Sprint(len(partial)), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "manifest.next"), []byte(partial), 0600); err != nil {
				t.Fatal(err)
			}
			a := open(t, dir)
			accepted(t, a, fixture(t, "first", false))
		})
	}
	dir := filepath.Join(t.TempDir(), "shared")
	os.Mkdir(dir, 0755)
	if a, err := Open(dir); !errors.Is(err, ErrCorrupt) {
		if a != nil {
			a.Close()
		}
		t.Fatal("shared directory accepted", err)
	}
}

func TestC05AtomicVisibleSnapshot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	a := open(t, dir)
	accepted(t, a, fixture(t, "base", false))
	before, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil {
		t.Fatal(err)
	}
	checkedBefore, checkedAfter := false, false
	a.checkpoint = func(p string) error {
		if p != "manifest.publish" && p != "directory.sync" {
			return nil
		}
		visible, err := os.ReadFile(filepath.Join(dir, "manifest"))
		if err != nil {
			t.Fatal(err)
		}
		if p == "manifest.publish" {
			checkedBefore = true
			if !bytes.Equal(before, visible) {
				t.Fatal("uncommitted state became visible")
			}
		} else {
			checkedAfter = true
			if !bytes.HasPrefix(visible, before) || bytes.Equal(before, visible) {
				t.Fatal("snapshot mixed old and new state")
			}
			count := 0
			if _, err := a.scan(context.Background(), func(_ entry, v contracts.Input) error {
				count++
				for _, f := range v.Files {
					if _, err := a.object(context.Background(), f, false); err != nil {
						return err
					}
				}
				return nil
			}); err != nil || count != 2 {
				t.Fatal("published snapshot lacks durable references", err)
			}
		}
		return nil
	}
	accepted(t, a, fixture(t, "next", true))
	if !checkedBefore || !checkedAfter {
		t.Fatal("publication checkpoints missing")
	}
}
