package tests

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestC30DurableCLI(t *testing.T) {
	binary := buildC20CLI(t)
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	fixture := "fixtures/attestara/C30"
	args := []string{"evaluate", "--input", filepath.Join(fixture, "manifest.json"), "--policy", filepath.Join(fixture, "rule.json"), "--evidence", filepath.Join(fixture, "evidence.json"), "--freshness", "fresh", "--files", fixture, "--store", store, "--operation-id", "release-1"}
	first := runC20CLI(t, binary, args...)
	if first.code != 0 {
		t.Fatalf("durable evaluate: %d %s", first.code, first.stderr)
	}
	if again := runC20CLI(t, binary, args...); again.code != 0 || again.stdout != first.stdout {
		t.Fatalf("replay differs: %+v", again)
	}
	loaded := runC20CLI(t, binary, "verify", "--store", store, "--operation-id", "release-1")
	if loaded.code != 0 || loaded.stdout != first.stdout {
		t.Fatalf("fresh process differs: %+v", loaded)
	}
	backup := filepath.Join(dir, "backup")
	if got := runC20CLI(t, binary, "export", "--store", store, "--operation-id", "release-1", "--output", backup); got.code != 0 {
		t.Fatalf("backup: %+v", got)
	}
	restored := filepath.Join(dir, "restored")
	if got := runC20CLI(t, binary, "restore", "--backup", backup, "--store", restored, "--operation-id", "release-1"); got.code != 0 || got.stdout != first.stdout {
		t.Fatalf("restore: %+v", got)
	}
	if got := runC20CLI(t, binary, "restore", "--backup", backup, "--store", store, "--operation-id", "release-1"); got.code != 3 {
		t.Fatalf("existing target not rejected: %+v", got)
	}
	args[8] = "stale"
	if got := runC20CLI(t, binary, args...); got.code != 3 {
		t.Fatalf("changed replay not conflict: %+v", got)
	}
	if got := runC20CLI(t, binary, "verify", "--store", store, "--operation-id", "release-1"); got.stdout != first.stdout {
		t.Fatal("conflict changed accepted state")
	}
	if err := os.WriteFile(filepath.Join(backup, "manifest"), []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, "bad")
	if got := runC20CLI(t, binary, "restore", "--backup", backup, "--store", bad, "--operation-id", "release-1"); got.code != 3 {
		t.Fatalf("corrupt backup: %+v", got)
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("failed restore published target")
	}
}

func TestC30BundleTrustAndBinding(t *testing.T) {
	binary := buildC20CLI(t)
	fixture := "fixtures/attestara/C30"
	dir := t.TempDir()
	args := []string{"evaluate", "--input", filepath.Join(fixture, "manifest.json"), "--policy", filepath.Join(fixture, "rule.json"), "--evidence", filepath.Join(fixture, "evidence.json"), "--freshness", "fresh", "--files", fixture}
	first := runC20CLI(t, binary, args...)
	if first.code != 0 {
		t.Fatalf("evaluate: %+v", first)
	}
	passport := filepath.Join(dir, "passport.json")
	os.WriteFile(passport, []byte(first.stdout), 0600)
	base := []string{"verify", "--input", filepath.Join(fixture, "manifest.json"), "--files", fixture, "--passport", passport}
	for _, tc := range []struct {
		name, signature, trust string
		code                   int
		trusted                bool
	}{
		{"unsigned", "", "", 0, false},
		{"untrusted", "signature.json", "untrusted.json", 0, false},
		{"trusted", "signature.json", "trust.json", 0, true},
		{"invalid", "invalid-signature.json", "trust.json", 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := append([]string{}, base...)
			if tc.signature != "" {
				a = append(a, "--signature", filepath.Join(fixture, tc.signature))
			}
			if tc.trust != "" {
				a = append(a, "--trust", filepath.Join(fixture, tc.trust))
			}
			got := runC20CLI(t, binary, a...)
			if got.code != tc.code {
				t.Fatalf("assessment: %+v", got)
			}
			if tc.code == 0 {
				if got.stdout != first.stdout {
					t.Fatal("assessment rewrote passport")
				}
				var result struct {
					Trusted bool `json:"manifest_trusted"`
				}
				if err := json.Unmarshal([]byte(got.stderr), &result); err != nil || result.Trusted != tc.trusted {
					t.Fatalf("assessment: %+v", got)
				}
			}
		})
	}
	zipPath := filepath.Join(dir, "release.zip")
	if got := runC20CLI(t, binary, "export", "--input", filepath.Join(fixture, "manifest.json"), "--files", fixture, "--output", zipPath); got.code != 0 {
		t.Fatal(got)
	}
	if got := runC20CLI(t, binary, "verify", "--archive", zipPath, "--passport", passport, "--signature", filepath.Join(fixture, "signature.json"), "--trust", filepath.Join(fixture, "trust.json")); got.code != 0 || got.stdout != first.stdout {
		t.Fatalf("offline parity: %+v", got)
	}
	mutated := filepath.Join(dir, "files")
	os.Mkdir(mutated, 0700)
	for _, name := range []string{"subject.txt", "rule.json", "evidence.json", "sbom.cdx.json"} {
		b, _ := os.ReadFile(filepath.Join(fixture, name))
		os.WriteFile(filepath.Join(mutated, name), b, 0600)
	}
	os.WriteFile(filepath.Join(mutated, "subject.txt"), []byte("changed"), 0600)
	if got := runC20CLI(t, binary, "verify", "--input", filepath.Join(fixture, "manifest.json"), "--files", mutated); got.code != 2 {
		t.Fatalf("changed subject accepted: %+v", got)
	}
	args[8] = "stale"
	if got := runC20CLI(t, binary, args...); got.code != 1 || c20Passport(t, got.stdout).PolicyDecision != "unknown" {
		t.Fatalf("stale: %+v", got)
	}
}

func TestC30MalformedAndMissingEvidenceCannotBeAccepted(t *testing.T) {
	binary := buildC20CLI(t)
	fixture := "fixtures/attestara/C30"
	for _, tc := range []struct {
		name   string
		mutate func(string, map[string]any)
	}{
		{"incomplete-manifest", func(dir string, m map[string]any) { delete(m, "evidence_digest") }},
		{"missing-file", func(dir string, m map[string]any) { os.Remove(filepath.Join(dir, "sbom.cdx.json")) }},
		{"escaping-link", func(dir string, m map[string]any) {
			os.Remove(filepath.Join(dir, "subject.txt"))
			os.Symlink("/etc/passwd", filepath.Join(dir, "subject.txt"))
		}},
		{"malformed-sbom", func(dir string, m map[string]any) {
			b := []byte(`{"bomFormat":"CycloneDX","specVersion":"1.7","version":"bad"}`)
			os.WriteFile(filepath.Join(dir, "sbom.cdx.json"), b, 0600)
			for _, f := range m["files"].([]any) {
				entry := f.(map[string]any)
				if entry["path"] == "sbom.cdx.json" {
					hash := sha256.Sum256(b)
					entry["digest"] = "sha256:" + hex.EncodeToString(hash[:])
					entry["size_bytes"] = len(b)
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"subject.txt", "rule.json", "evidence.json", "sbom.cdx.json"} {
				b, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				os.WriteFile(filepath.Join(dir, name), b, 0600)
			}
			b, _ := os.ReadFile(filepath.Join(fixture, "manifest.json"))
			var m map[string]any
			json.Unmarshal(b, &m)
			tc.mutate(dir, m)
			b, _ = json.Marshal(m)
			manifest := filepath.Join(dir, "manifest.json")
			os.WriteFile(manifest, b, 0600)
			store := filepath.Join(dir, "store")
			result := runC20CLI(t, binary, "evaluate", "--input", manifest, "--policy", filepath.Join(dir, "rule.json"), "--evidence", filepath.Join(dir, "evidence.json"), "--freshness", "fresh", "--files", dir, "--store", store, "--operation-id", "bad")
			if result.code == 0 || result.code == 1 || result.stdout != "" {
				t.Fatalf("invalid input accepted: %+v", result)
			}
			if _, err := os.Stat(store); !os.IsNotExist(err) {
				t.Fatal("invalid input initialized store")
			}
		})
	}
	missing := runC20CLI(t, binary, "evaluate", "--input", filepath.Join(fixture, "manifest.json"), "--policy", filepath.Join(fixture, "rule.json"), "--freshness", "fresh")
	if missing.code != 1 || c20Passport(t, missing.stdout).PolicyDecision != "unknown" {
		t.Fatalf("missing evidence passed: %+v", missing)
	}
}

func TestC30SourceStaysInertAndPublicBindingUsesRedaction(t *testing.T) {
	binary := buildC20CLI(t)
	dir := t.TempDir()
	fixture := "fixtures/attestara/C30"
	marker := filepath.Join(dir, "NEVER_EXECUTED")
	payload := []byte("#!/bin/sh\ntouch " + marker + "\n")
	for _, name := range []string{"rule.json", "evidence.json", "sbom.cdx.json"} {
		b, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "subject.txt"), payload, 0700)
	b, _ := os.ReadFile(filepath.Join(fixture, "manifest.json"))
	var m map[string]any
	json.Unmarshal(b, &m)
	hash := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(hash[:])
	m["subject_digest"] = digest
	m["release_id"] = "token=C30_SYNTHETIC_SECRET"
	for _, item := range m["files"].([]any) {
		f := item.(map[string]any)
		if f["path"] == "subject.txt" {
			f["digest"] = digest
			f["size_bytes"] = len(payload)
		}
	}
	b, _ = json.Marshal(m)
	manifest := filepath.Join(dir, "manifest.json")
	os.WriteFile(manifest, b, 0600)
	got := runC20CLI(t, binary, "evaluate", "--input", manifest, "--files", dir, "--policy", filepath.Join(dir, "rule.json"), "--evidence", filepath.Join(dir, "evidence.json"), "--freshness", "fresh")
	if got.code != 0 || c20Passport(t, got.stdout).ReleaseID != "[REDACTED]" {
		t.Fatalf("evaluation: %+v", got)
	}
	passport := filepath.Join(dir, "passport.json")
	os.WriteFile(passport, []byte(got.stdout), 0600)
	if verified := runC20CLI(t, binary, "verify", "--input", manifest, "--files", dir, "--passport", passport); verified.code != 0 || verified.stdout != got.stdout {
		t.Fatalf("redacted binding: %+v", verified)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("source executed")
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if blocked := runC20CLI(t, binary, "verify", "--passport", filepath.Join(dir, "fifo")); blocked.code != 2 {
		t.Fatalf("nonregular input accepted: %+v", blocked)
	}
}
