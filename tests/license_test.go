package tests

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"attestara/internal/contracts"
	"attestara/internal/licensecatalog"
	"attestara/internal/policy"
)

func readC13(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("fixtures/attestara/C13", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestC13RecognizedAndReviewExpressions(t *testing.T) {
	var input struct {
		SchemaVersion int      `json:"schema_version"`
		Expressions   []string `json:"expressions"`
	}
	var expected struct {
		SchemaVersion int                        `json:"schema_version"`
		Assessments   []policy.LicenseAssessment `json:"assessments"`
	}
	if err := json.Unmarshal(readC13(t, "positive.json"), &input); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(readC13(t, "expected.json"), &expected); err != nil {
		t.Fatal(err)
	}
	if input.SchemaVersion != 1 || expected.SchemaVersion != 1 || len(input.Expressions) != len(expected.Assessments) || len(input.Expressions) == 0 {
		t.Fatal("invalid fixture contract")
	}
	for i, expression := range input.Expressions {
		t.Run(expression, func(t *testing.T) {
			got, err := policy.AssessLicenseExpression(expression)
			if err != nil || got != expected.Assessments[i] {
				t.Fatalf("got %+v, %v; want %+v", got, err, expected.Assessments[i])
			}
			b, _ := json.Marshal(got)
			if bytes.Contains(b, []byte("compliant")) || bytes.Contains(b, []byte("allow")) {
				t.Fatal("recognition claimed compliance")
			}
		})
	}
}

func TestC13RejectsMalformedAndBoundedInput(t *testing.T) {
	var negative struct {
		Cases []struct {
			ID, Expression string
			Code           string `json:"expected_code"`
		}
	}
	if err := json.Unmarshal(readC13(t, "negative.json"), &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Cases) == 0 {
		t.Fatal("negative fixtures missing")
	}
	reject := func(expression, code string) {
		t.Helper()
		got, err := policy.AssessLicenseExpression(expression)
		var typed *contracts.Error
		if got != (policy.LicenseAssessment{}) || !errors.As(err, &typed) || typed.Code != code || typed.Field != "expression" || typed.Message == "" {
			t.Fatalf("got %+v, %v; want empty result and %s", got, err, code)
		}
		if strings.Contains(typed.Message, "sensitive-value") {
			t.Fatal("error disclosed expression")
		}
	}
	for _, tc := range negative.Cases {
		t.Run(tc.ID, func(t *testing.T) { reject(tc.Expression, tc.Code) })
	}
	reject("MIT AND sensitive-value!", "invalid_input")
	reject(string([]byte{0xff}), "invalid_input")
	reject(strings.Repeat("a", 4097), "limit_exceeded")
	for _, expression := range []string{strings.Repeat("a", 4096), strings.Repeat("(", 2046) + "MIT" + strings.Repeat(")", 2046)} {
		if _, err := policy.AssessLicenseExpression(expression); err != nil {
			t.Fatal("bounded valid input rejected", err)
		}
	}
	sentinel := filepath.Join(t.TempDir(), "executed")
	_, _ = policy.AssessLicenseExpression("touch " + sentinel)
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("expression executed", err)
	}
}

func TestC13SharedCatalogAndExceptionRoles(t *testing.T) {
	data := licensecatalog.Schema()
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != "4b345e2329f209f34e960ae2a8e7cb46a166907e6a45e94978565925dc47b359" {
		t.Fatal("shared schema pin changed")
	}
	var catalog struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Enum) != 826 {
		t.Fatal("catalog incomplete")
	}
	for i, id := range catalog.Enum {
		want := "license"
		expression := id
		if i >= 740 {
			want = "exception"
			expression = "MIT WITH " + id
		}
		kind, err := licensecatalog.Kind(strings.ToLower(id))
		if err != nil || kind != want {
			t.Fatalf("wrong category for %s: %s %v", id, kind, err)
		}
		got, err := policy.AssessLicenseExpression(expression)
		if err != nil || got.Status != "recognized" {
			t.Fatal("catalog expression not recognized", expression, err)
		}
	}
	data[0] = ' '
	if licensecatalog.Schema()[0] != '{' {
		t.Fatal("catalog aliases caller data")
	}
}

func TestC13FreshProcessRepeat(t *testing.T) {
	var input struct {
		Expressions []string `json:"expressions"`
	}
	if err := json.Unmarshal(readC13(t, "positive.json"), &input); err != nil {
		t.Fatal(err)
	}
	var results []policy.LicenseAssessment
	for _, expression := range input.Expressions {
		got, err := policy.AssessLicenseExpression(expression)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, got)
	}
	data, _ := json.Marshal(results)
	digest := sha256.Sum256(data)
	want := hex.EncodeToString(digest[:])
	if os.Getenv("ATTESTARA_C13_HELPER") == "1" {
		t.Log(want)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestC13FreshProcessRepeat$", "-test.v")
	cmd.Env = append(os.Environ(), "ATTESTARA_C13_HELPER=1")
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), want) {
		t.Fatal("fresh process changed assessment", string(out), err)
	}
}
