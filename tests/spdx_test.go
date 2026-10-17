package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"attestara/internal/adapters"
	"attestara/internal/bundle"
	"attestara/internal/contracts"
)

func readC11(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C11", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func TestC11NormalizedRoundTrip(t *testing.T) {
	data := readC11(t, "positive.json")
	before := bytes.Clone(data)
	got, err := adapters.DecodeSPDX(data)
	if err != nil {
		t.Fatal(err)
	}
	var expected adapters.NormalizedBOM
	if err := json.Unmarshal(readC11(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.BOM, expected) {
		t.Fatalf("got %+v; want %+v", got.BOM, expected)
	}
	normalized, err := json.Marshal(got.BOM)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adapters.DecodeCycloneDX(normalized); err != nil {
		t.Fatal("normalized output is not CycloneDX 1.7", err)
	}
	exported, err := adapters.EncodeSPDX(got)
	if err != nil {
		t.Fatal(err)
	}
	again, err := adapters.DecodeSPDX(exported)
	if err != nil || !reflect.DeepEqual(again.BOM, got.BOM) {
		t.Fatal("round trip changed package IDs or dependency direction", err)
	}
	repeated, err := adapters.DecodeSPDX(data)
	if err != nil || !reflect.DeepEqual(repeated.BOM, got.BOM) {
		t.Fatal("repeat differs", err)
	}
	if !bytes.Equal(data, before) {
		t.Fatal("input mutated")
	}
	got.BOM.Components[0].Name = "Changed normalized name"
	exported, err = adapters.EncodeSPDX(got)
	if err != nil {
		t.Fatal(err)
	}
	again, err = adapters.DecodeSPDX(exported)
	if err != nil || again.BOM.Components[0].Name != "Changed normalized name" {
		t.Fatal("export bypassed normalized projection", err)
	}
	data[0] = ' '
	if _, err := adapters.EncodeSPDX(repeated); err != nil {
		t.Fatal("result aliases input bytes", err)
	}
	if repeated.BOM.Components[0].Name == got.BOM.Components[0].Name {
		t.Fatal("results alias")
	}
}
func TestC11RejectsInvalidFixtures(t *testing.T) {
	var negative struct {
		Cases []struct {
			ID, Input string
			Code      string `json:"expected_code"`
			Field     string `json:"expected_field"`
		}
	}
	if err := json.Unmarshal(readC11(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	prior, err := adapters.DecodeSPDX(readC11(t, "positive.json"))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(prior.BOM)
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) {
			got, err := adapters.DecodeSPDX([]byte(tc.Input))
			var typed *contracts.Error
			if !reflect.DeepEqual(got, adapters.SPDX{}) || !errors.As(err, &typed) || typed.Code != tc.Code || typed.Field != tc.Field || typed.Message == "" {
				t.Fatalf("got %+v, %v; want %s/%s", got, err, tc.Code, tc.Field)
			}
		})
	}
	after, _ := json.Marshal(prior.BOM)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected read altered prior projection")
	}
}
func TestC11BoundsInertInputAndArchiveBoundary(t *testing.T) {
	for _, data := range [][]byte{make([]byte, contracts.MaxBundleBytes+1), []byte("{\"x\":\"\xff\"}")} {
		_, err := adapters.DecodeSPDX(data)
		if err == nil {
			t.Fatal("hostile input accepted")
		}
	}
	sentinel := filepath.Join(t.TempDir(), "executed")
	data := bytes.ReplaceAll(readC11(t, "positive.json"), []byte("inert source text"), []byte("touch "+sentinel))
	got, err := adapters.DecodeSPDX(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adapters.EncodeSPDX(got); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("source executed", err)
	}
	for _, name := range []string{"path-0.zip", "symlink.zip", "symlink-directory.zip"} {
		b, err := os.ReadFile(filepath.Join("fixtures/attestara/C09", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = bundle.ReadArchive(b); err == nil {
			t.Fatal("unsafe archive admitted", name)
		}
	}
	// Caller edits are validated at the export boundary as well.
	got.BOM.Components[0].Type = "unmapped"
	if b, err := adapters.EncodeSPDX(got); err == nil || b != nil {
		t.Fatal("invalid projection exported")
	}
}

func TestC11PurposesAndMergedDependencies(t *testing.T) {
	purposes := map[string]string{"application": "application", "framework": "framework", "library": "library", "container": "container", "operatingSystem": "operating-system", "device": "device", "deviceDriver": "device-driver", "firmware": "firmware", "file": "file", "data": "data", "model": "machine-learning-model", "platform": "platform"}
	for purpose, typ := range purposes {
		t.Run(purpose, func(t *testing.T) {
			data := bytes.ReplaceAll(readC11(t, "positive.json"), []byte(`"software_primaryPurpose": "application"`), []byte(`"software_primaryPurpose": "`+purpose+`"`))
			got, err := adapters.DecodeSPDX(data)
			if err != nil || got.BOM.Components[0].Type != typ {
				t.Fatal("incorrect purpose mapping", err)
			}
			b, err := adapters.EncodeSPDX(got)
			if err != nil {
				t.Fatal(err)
			}
			again, err := adapters.DecodeSPDX(b)
			if err != nil || !reflect.DeepEqual(again.BOM, got.BOM) {
				t.Fatal("purpose lost in round trip", err)
			}
		})
	}
	var doc map[string]any
	if err := json.Unmarshal(readC11(t, "positive.json"), &doc); err != nil {
		t.Fatal(err)
	}
	graph := doc["@graph"].([]any)
	graph = append(graph, map[string]any{"type": "Relationship", "spdxId": "urn:c11:second", "creationInfo": "_:creation", "from": "urn:c11:app", "to": []string{"urn:c11:app", "urn:c11:lib"}, "relationshipType": "dependsOn"})
	doc["@graph"] = graph
	b, _ := json.Marshal(doc)
	got, err := adapters.DecodeSPDX(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.BOM.Dependencies) != 1 || !reflect.DeepEqual(got.BOM.Dependencies[0].DependsOn, []string{"urn:c11:app", "urn:c11:lib"}) {
		t.Fatal("graph was not merged deterministically")
	}
	if len(got.BOM.Dependencies) != 1 {
		t.Fatal("unknown dependencies turned into empty assertions")
	}
	b, err = adapters.EncodeSPDX(got)
	if err != nil {
		t.Fatal(err)
	}
	again, err := adapters.DecodeSPDX(b)
	if err != nil || !reflect.DeepEqual(again.BOM, got.BOM) {
		t.Fatal("merged graph lost", err)
	}
}

func TestC11NeverFetchesContextOrMetadata(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	data := bytes.ReplaceAll(readC11(t, "positive.json"), []byte("inert source text"), []byte(server.URL+"/metadata"))
	if _, err := adapters.DecodeSPDX(data); err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("https://spdx.org/rdf/3.0.0/spdx-context.jsonld"), []byte(server.URL+"/context"))
	if _, err := adapters.DecodeSPDX(data); err == nil {
		t.Fatal("foreign context accepted")
	}
	if requests.Load() != 0 {
		t.Fatal("untrusted URL fetched")
	}
}

func TestC11InterruptedRead(t *testing.T) {
	if os.Getenv("ATTESTARA_C11_READ_HELPER") == "1" {
		data := readC11(t, "positive.json")
		if _, err := adapters.DecodeSPDX(data); err != nil {
			t.Fatal(err)
		}
		fmt.Println("ready")
		for {
			if _, err := adapters.DecodeSPDX(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	data := readC11(t, "positive.json")
	prior, err := adapters.DecodeSPDX(data)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestC11InterruptedRead$")
	cmd.Env = append(os.Environ(), "ATTESTARA_C11_READ_HELPER=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(out)
	ready := scanner.Scan() && scanner.Text() == "ready"
	killErr := cmd.Process.Kill()
	waitErr := cmd.Wait()
	if !ready || killErr != nil || waitErr == nil || ctx.Err() != nil {
		t.Fatal("interruption not demonstrated", killErr, waitErr, ctx.Err())
	}
	after := readC11(t, "positive.json")
	again, err := adapters.DecodeSPDX(after)
	if err != nil || !bytes.Equal(data, after) || !reflect.DeepEqual(prior.BOM, again.BOM) {
		t.Fatal("interrupted read changed prior bytes or repeat", err)
	}
}
