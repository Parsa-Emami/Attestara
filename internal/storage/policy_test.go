package storage

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"attestara/internal/policy"
)

func policyRecord(t *testing.T, id string) Record {
	t.Helper()
	read := func(name string) []byte {
		b, err := os.ReadFile("../../tests/fixtures/attestara/C14/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	r, err := policy.Evaluate(context.Background(), read("input.json"), read("rule.json"), read("evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return Record{OperationID: id, InputJSON: read("input.json"), ResultJSON: b, Files: map[string][]byte{"subject.txt": read("subject.txt"), "rule.json": read("rule.json"), "evidence.json": read("evidence.json")}}
}
func TestC14CrashHelper(t *testing.T) {
	dir := os.Getenv("ATTESTARA_C14_TEST_DIR")
	if dir == "" {
		return
	}
	a := open(t, dir)
	phase := os.Getenv("ATTESTARA_C14_TEST_PHASE")
	a.checkpoint = func(p string) error {
		if p == phase {
			syscall.Kill(os.Getpid(), syscall.SIGKILL)
		}
		return nil
	}
	if err := a.Commit(context.Background(), policyRecord(t, "next")); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash point not reached")
}
func TestC14V2CrashRecovery(t *testing.T) {
	for _, phase := range negative(t).Crash {
		t.Run(phase, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			a := open(t, dir)
			base := fixture(t, "base", false)
			accepted(t, a, base)
			a.Close()
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestC14CrashHelper$")
			cmd.Env = append(os.Environ(), "ATTESTARA_C14_TEST_DIR="+dir, "ATTESTARA_C14_TEST_PHASE="+phase)
			b, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if ctx.Err() != nil || !errors.As(err, &exit) {
				t.Fatalf("helper did not crash: %v %s", err, b)
			}
			status, ok := exit.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("unexpected helper exit: %v %s", err, b)
			}
			a = open(t, dir)
			next := policyRecord(t, "next")
			assertLoad(t, a, base, true)
			assertLoad(t, a, next, published(phase))
			accepted(t, a, next)
			before, err := os.ReadFile(filepath.Join(dir, "manifest"))
			if err != nil {
				t.Fatal(err)
			}
			accepted(t, a, next)
			after, _ := os.ReadFile(filepath.Join(dir, "manifest"))
			if !bytes.Equal(before, after) {
				t.Fatal("recovery/replay duplicated v2")
			}
		})
	}
}
func TestC14V2DiskFullAndLostACK(t *testing.T) {
	for _, phase := range negative(t).IO {
		t.Run(phase, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "store")
			a := open(t, dir)
			base := fixture(t, "base", false)
			accepted(t, a, base)
			next := policyRecord(t, "next")
			fired := false
			a.checkpoint = func(p string) error {
				if p == phase {
					fired = true
					return syscall.ENOSPC
				}
				return nil
			}
			if err := a.Commit(context.Background(), next); !fired || !errors.Is(err, syscall.ENOSPC) {
				t.Fatal("fault not exercised", err)
			}
			if _, err := a.Load(context.Background(), "base"); !errors.Is(err, ErrRecovery) {
				t.Fatal("uncertain adapter reused", err)
			}
			a.Close()
			a = open(t, dir)
			assertLoad(t, a, base, true)
			assertLoad(t, a, next, published(phase))
			accepted(t, a, next)
			before, _ := os.ReadFile(filepath.Join(dir, "manifest"))
			accepted(t, a, next)
			after, _ := os.ReadFile(filepath.Join(dir, "manifest"))
			if !bytes.Equal(before, after) {
				t.Fatal("lost ACK replay duplicated transaction")
			}
		})
	}
}
func TestC14V2QuotaPreservesSnapshot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	a := open(t, dir)
	seed := policyRecord(t, "seed")
	accepted(t, a, seed)
	a.Close()
	// Seed the real on-disk manifest at the supported operation limit without
	// performing 1000 quadratic rewrites. Objects came from a real v2 commit.
	manifest := bytes.NewBufferString(header)
	for i := 0; i < maxOperations; i++ {
		e := entry{OperationID: fmt.Sprintf("c14-%04d", i), Input: seed.InputJSON, Result: seed.ResultJSON}
		e.Checksum = checksum(e)
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		manifest.Write(b)
		manifest.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest"), manifest.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	a = open(t, dir)
	if err := a.Commit(context.Background(), policyRecord(t, "overflow")); !errors.Is(err, ErrQuota) {
		t.Fatal("quota did not reject", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "manifest"))
	if err != nil || !bytes.Equal(after, manifest.Bytes()) {
		t.Fatal("quota changed accepted bytes", err)
	}
	seed.OperationID = "c14-0000"
	assertLoad(t, a, seed, true)
}
