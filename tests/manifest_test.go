package tests

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"attestara/internal/bundle"
	"attestara/internal/contracts"
)

const c08Fixtures = "fixtures/attestara/C08"

type c08Negative struct {
	Cases []struct {
		ID            string `json:"id"`
		Mutation      string `json:"mutation"`
		File          string `json:"file,omitempty"`
		Field         string `json:"field,omitempty"`
		ExpectedCode  string `json:"expected_code"`
		ExpectedField string `json:"expected_field"`
	} `json:"cases"`
}

func readC08(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(c08Fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func copyC08(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	entries, err := os.ReadDir(c08Fixtures)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c08Fixtures, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, entry.Name()), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestC08ReadsVersionedManifestAndRealFiles(t *testing.T) {
	var expected struct {
		SchemaVersion string `json:"schema_version"`
		ReleaseID     string `json:"release_id"`
		Verification  string `json:"verification"`
		Files         []struct {
			Path      string `json:"path"`
			Digest    string `json:"digest"`
			SizeBytes int64  `json:"size_bytes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(readC08(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	got, err := bundle.Read(readC08(t, "positive.json"), os.DirFS(c08Fixtures))
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != expected.SchemaVersion || got.ReleaseID != expected.ReleaseID || got.Verification != "unverified" || got.Verification != expected.Verification || len(got.Files) != len(expected.Files) {
		t.Fatalf("input = %+v; want expected typed manifest %+v", got, expected)
	}
	for i, file := range got.Files {
		if file.Path != expected.Files[i].Path || file.Digest != expected.Files[i].Digest || *file.SizeBytes != expected.Files[i].SizeBytes {
			t.Errorf("file[%d] = %+v; want %+v", i, file, expected.Files[i])
		}
	}
}

func TestC08RejectsManifestAndFileNegatives(t *testing.T) {
	var negative c08Negative
	if err := json.Unmarshal(readC08(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) == 0 {
		t.Fatal("negative fixture has no cases")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			root := copyC08(t)
			var input contracts.Input
			if err := json.Unmarshal(readC08(t, "positive.json"), &input); err != nil {
				t.Fatal(err)
			}
			switch tc.Mutation {
			case "remove_file":
				if err := os.Remove(filepath.Join(root, tc.File)); err != nil {
					t.Fatal(err)
				}
			case "duplicate_path":
				input.Files[1].Path = input.Files[0].Path
			case "size_plus_one":
				*input.Files[1].SizeBytes++
			case "modify_byte":
				path := filepath.Join(root, tc.File)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				data[0] ^= 1
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			case "remove_digest_file":
				input.Files = input.Files[:1]
			case "trusted":
				input.Verification = "trusted"
			case "add_signer", "add_expiry":
				var value map[string]any
				if err := json.Unmarshal(readC08(t, "positive.json"), &value); err != nil {
					t.Fatal(err)
				}
				if tc.Mutation == "add_signer" {
					value["signer"] = map[string]any{"id": "untrusted-fixture-signer"}
				} else {
					value["expires_at"] = "1970-01-01T00:00:00Z"
				}
				encoded, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				err = assertC08Rejected(t, encoded, os.DirFS(root), tc.ExpectedCode, tc.ExpectedField)
				if err != nil {
					t.Fatal(err)
				}
				return
			default:
				t.Fatalf("unknown fixture mutation %q", tc.Mutation)
			}
			encoded, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := assertC08Rejected(t, encoded, os.DirFS(root), tc.ExpectedCode, tc.ExpectedField); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestC08RejectsUnavailableFileSource(t *testing.T) {
	if err := assertC08Rejected(t, readC08(t, "positive.json"), nil, "input_unavailable", "files"); err != nil {
		t.Fatal(err)
	}
}

func assertC08Rejected(t *testing.T, manifest []byte, files fs.FS, code, field string) error {
	t.Helper()
	got, err := bundle.Read(manifest, files)
	if err == nil {
		return errors.New("invalid bundle was accepted")
	}
	if got.ReleaseID != "" || got.Verification != "" {
		return errors.New("invalid bundle returned a partial typed result")
	}
	var contractErr *contracts.Error
	if !errors.As(err, &contractErr) || contractErr.Code != code || contractErr.Field != field || contractErr.Message == "" {
		return errors.New("bundle returned an unexpected or unsafe error")
	}
	return nil
}

func TestC08ReplayStaysUnverifiedAndDoesNotChangeFiles(t *testing.T) {
	root := copyC08(t)
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest := readC08(t, "positive.json")
	first, err := bundle.Read(manifest, os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	second, err := bundle.Read(manifest, os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	var stale contracts.Input
	if err := json.Unmarshal(manifest, &stale); err != nil {
		t.Fatal(err)
	}
	stale.SourceRevision = "synthetic-c08-revision-0"
	staleManifest, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	older, err := bundle.Read(staleManifest, os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if first.Verification != "unverified" || second.Verification != "unverified" || older.Verification != "unverified" || !reflect.DeepEqual(first, second) || older.SourceRevision == first.SourceRevision {
		t.Fatalf("replay/stale read changed trust or input: first=%+v second=%+v older=%+v", first, second, older)
	}
	after, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("read changed fixture filesystem: before=%v after=%v", before, after)
	}
}
