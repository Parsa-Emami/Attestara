package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"attestara/internal/adapters"
	"attestara/internal/contracts"
)

func readC10(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C10", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func rejectC10(t *testing.T, data []byte, code, field string) {
	t.Helper()
	got, err := adapters.DecodeCycloneDX(data)
	var typed *contracts.Error
	if !reflect.DeepEqual(got, adapters.CycloneDX{}) || !errors.As(err, &typed) || typed.Code != code || typed.Field != field || typed.Message == "" {
		t.Fatalf("got %+v, %v; want empty result and %s/%s", got, err, code, field)
	}
	if bytes.Contains([]byte(typed.Message), []byte("sensitive-value")) {
		t.Fatal("error disclosed input")
	}
}

func TestC10ValidatesAndPreservesOriginalBytes(t *testing.T) {
	var expected struct {
		BOMFormat   string `json:"bomFormat"`
		SpecVersion string `json:"specVersion"`
	}
	if err := json.Unmarshal(readC10(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	data := readC10(t, "positive.json")
	before := bytes.Clone(data)
	got, err := adapters.DecodeCycloneDX(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.BOMFormat != expected.BOMFormat || got.SpecVersion != expected.SpecVersion || !bytes.Equal(got.Raw, before) || !bytes.Equal(data, before) {
		t.Fatal("header or original JSON bytes changed")
	}
	second, err := adapters.DecodeCycloneDX(data)
	if err != nil || !reflect.DeepEqual(got, second) {
		t.Fatal("repeated decoding changed result", err)
	}
	data[0] = ' '
	if !bytes.Equal(got.Raw, before) {
		t.Fatal("result aliases caller input")
	}
	got.Raw[0] = ' '
	if !bytes.Equal(second.Raw, before) {
		t.Fatal("results share mutable storage")
	}
	legacy, err := os.ReadFile("fixtures/attestara/C04/sbom.cdx.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adapters.DecodeCycloneDX(legacy); err != nil {
		t.Fatal("existing C04 BOM rejected", err)
	}
}

func TestC10RejectsInvalidFixtures(t *testing.T) {
	var negative struct {
		Cases []struct {
			ID    string `json:"id"`
			Input string `json:"input"`
			Code  string `json:"expected_code"`
			Field string `json:"expected_field"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(readC10(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) == 0 {
		t.Fatal("negative fixtures missing")
	}
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) { rejectC10(t, []byte(tc.Input), tc.Code, tc.Field) })
	}
}

func TestC10LimitsStrictUTF8AndInertInput(t *testing.T) {
	rejectC10(t, make([]byte, contracts.MaxBundleBytes+1), "limit_exceeded", "$")
	rejectC10(t, []byte("{\"bomFormat\":\"CycloneDX\",\"specVersion\":\"1.7\",\"properties\":[{\"name\":\"x\",\"value\":\"\xff\"}]}"), "invalid_json", "$")
	// A valid BOM may describe arbitrary source text; it must remain inert.
	sentinel := filepath.Join(t.TempDir(), "executed")
	data := bytes.ReplaceAll(readC10(t, "positive.json"), []byte("#!/bin/sh; inert data only"), []byte("touch "+sentinel))
	if _, err := adapters.DecodeCycloneDX(data); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("source text was executed", err)
	}
}

func TestC10NeverLoadsDocumentURLs(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	data := bytes.ReplaceAll(readC10(t, "positive.json"), []byte("https://untrusted.invalid/never-fetch.json"), []byte(server.URL+"/schema.json"))
	if _, err := adapters.DecodeCycloneDX(data); err != nil {
		t.Fatal(err)
	}
	// Even a schema-invalid root $ref must never load a network or file resource.
	for _, location := range []string{server.URL + "/ref.json", "file:///etc/passwd", "../../outside.json"} {
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		doc["$ref"] = location
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		rejectC10(t, b, "invalid_input", "$")
	}
	if requests.Load() != 0 {
		t.Fatal("document URL was fetched")
	}
}

func TestC10EmbeddedSchemaPins(t *testing.T) {
	var provenance struct {
		SourceCommit string            `json:"source_commit"`
		Files        map[string]string `json:"files"`
	}
	root := filepath.Join("..", "internal", "adapters", "schemas")
	b, err := os.ReadFile(filepath.Join(root, "provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &provenance); err != nil {
		t.Fatal(err)
	}
	if provenance.SourceCommit != "349314a9d7671d7d2ca5b711a725f49a73979da6" || len(provenance.Files) != 4 {
		t.Fatal("C02 schema pin changed")
	}
	for name, expected := range provenance.Files {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != expected {
			t.Fatalf("schema bytes differ from pin: %s", name)
		}
	}
}
