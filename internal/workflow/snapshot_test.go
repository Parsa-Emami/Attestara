package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"attestara/internal/contracts"
	"attestara/internal/decision"
	"attestara/internal/policy"
	"attestara/internal/storage"
	json "encoding/json/v2"
)

func TestSnapshotPublicationRefusesExistingPathsAndCancellation(t *testing.T) {
	ctx := context.Background()
	fixture := "../../tests/fixtures/attestara/C30"
	read := func(name string) []byte {
		t.Helper()
		b, e := os.ReadFile(filepath.Join(fixture, name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	input := read("manifest.json")
	files, err := ReadFiles(ctx, input, fixture)
	if err != nil {
		t.Fatal(err)
	}
	result, err := policy.Evaluate(ctx, input, read("rule.json"), read("evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err = decision.Explain(ctx, result, decision.Fresh)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(result)
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	a, err := storage.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Commit(ctx, storage.Record{OperationID: "one", InputJSON: input, ResultJSON: b, Files: files}); err != nil {
		t.Fatal(err)
	}
	a.Close()
	for _, kind := range []string{"empty-directory", "symlink", "file"} {
		t.Run(kind, func(t *testing.T) {
			target := filepath.Join(dir, kind)
			switch kind {
			case "empty-directory":
				os.Mkdir(target, 0700)
			case "symlink":
				os.Symlink(source, target)
			case "file":
				os.WriteFile(target, []byte("keep"), 0600)
			}
			info, _ := os.Lstat(target)
			if _, err := Copy(ctx, source, "one", target); err == nil {
				t.Fatal("existing target replaced")
			}
			after, err := os.Lstat(target)
			if err != nil || !os.SameFile(info, after) {
				t.Fatal("existing target changed")
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	target := filepath.Join(dir, "canceled")
	if _, err = Copy(canceled, source, "one", target); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err = os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("cancellation published destination")
	}
	if _, err = Load(ctx, source, "one"); err != nil {
		t.Fatal("source changed")
	}
	var manifest contracts.Input
	json.Unmarshal(input, &manifest)
	if err = os.Remove(filepath.Join(source, "objects", manifest.SubjectDigest[7:])); err != nil {
		t.Fatal(err)
	}
	if _, err = Copy(ctx, source, "one", filepath.Join(dir, "missing-object")); err == nil {
		t.Fatal("missing evidence restored")
	}
}
