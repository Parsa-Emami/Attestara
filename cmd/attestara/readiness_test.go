package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"attestara/internal/storage"
)

func TestC30RealStorageReadinessAndLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store")
	a, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlerForStore(path)
	probe := func(code int, state string) {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
		if rec.Code != code || !strings.Contains(rec.Body.String(), `"storage":"`+state+`"`) {
			t.Fatalf("readiness: %d %s", rec.Code, rec.Body.String())
		}
	}
	probe(503, "locked")
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	probe(200, "ready")
	if err := os.WriteFile(filepath.Join(path, "manifest"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	probe(503, "corrupt")
}
