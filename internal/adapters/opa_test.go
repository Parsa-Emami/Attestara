package adapters

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"attestara/internal/contracts"
)

func TestOPAOutputValidation(t *testing.T) {
	for _, tc := range []struct {
		name, output, code string
	}{
		{"pass", `{"result":[{"expressions":[{"value":true}]}]}`, ""},
		{"fail", `{"result":[{"expressions":[{"value":false}]}]}`, ""},
		{"malformed", `{`, "invalid_json"},
		{"undefined", `{}`, "invalid_input"},
		{"wrong-type", `{"result":[{"expressions":[{"value":"secret"}]}]}`, "invalid_input"},
		{"duplicate", `{"result":[],"result":[]}`, "invalid_json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := opaDecision([]byte(tc.output))
			if tc.code == "" {
				if err != nil || got != (tc.name == "pass") {
					t.Fatalf("decision %v, %v", got, err)
				}
				return
			}
			var typed *contracts.Error
			if got || !errors.As(err, &typed) || typed.Code != tc.code || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe or incorrect result: %v, %v", got, err)
			}
		})
	}
}

func TestOPAProcessBoundsAndEnvironment(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"failed", "echo secret >&2; exit 1", "invalid_input"},
		{"stderr-limit", "while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' >&2; done", "limit_exceeded"},
		{"stdout-limit", "while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'; done", "limit_exceeded"},
		{"empty-env", `[ -z "$OPA_EVAL_FORMAT" ] || exit 1; printf 'ok'`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "provider")
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"+tc.body+"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("OPA_EVAL_FORMAT", "secret")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			out, err := runOPA(ctx, path, "", nil, "eval")
			if tc.code == "" {
				if err != nil || string(out) != "ok" {
					t.Fatalf("output %q, %v", out, err)
				}
				return
			}
			var typed *contracts.Error
			if !errors.As(err, &typed) || typed.Code != tc.code || len(out) != 0 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe or incorrect process result: %q, %v", out, err)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "provider")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := runOPA(ctx, path, "", nil, "eval")
	var typed *contracts.Error
	if !errors.As(err, &typed) || typed.Code != "deadline_exceeded" {
		t.Fatalf("timeout result: %v", err)
	}
}
