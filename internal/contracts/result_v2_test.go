package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestC14ResultVersionsAndSchema(t *testing.T) {
	var old map[string]any
	if err := json.Unmarshal(fixture(t, "result.valid.json"), &old); err != nil {
		t.Fatal(err)
	}
	// The old contract is still closed to all v2 fields and decisions.
	for _, value := range []any{nil, []any{}} {
		v := map[string]any{}
		json.Unmarshal(fixture(t, "result.valid.json"), &v)
		v["result"].(map[string]any)["evidence_references"] = value
		_, err := DecodeResult(context.Background(), encode(t, v))
		errorCode(t, err, "invalid_input")
	}
	escaped := bytes.Replace(encode(t, old), []byte(`"schema_version":"1"`), []byte(`"schema_version":"\u0031"`), 1)
	escaped = bytes.Replace(escaped, []byte(`"policy_decision":"unknown"`), []byte(`"policy_decision":"unknown","evidence_references":null`), 1)
	_, err := DecodeResult(context.Background(), escaped)
	errorCode(t, err, "invalid_input")
	old["schema_version"] = "3"
	_, err = DecodeResult(context.Background(), encode(t, old))
	errorCode(t, err, "unsupported_version")
	old["schema_version"] = "2"
	o := old["result"].(map[string]any)
	o["policy_decision"] = "pass"
	o["evidence_references"] = []any{map[string]any{"path": "evidence.json", "digest": o["evidence_digest"]}}
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonschema.SchemeURLLoader{})
	b, err := os.ReadFile("../../contracts/result.v2.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.AddResource("urn:parsa:attestara:result:2", schemaDoc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("urn:parsa:attestara:result:2")
	if err != nil {
		t.Fatal(err)
	}
	for _, decision := range []string{"pass", "fail", "unknown"} {
		o["policy_decision"] = decision
		wire := encode(t, old)
		if _, err := DecodeResult(context.Background(), wire); err != nil {
			t.Fatal(err)
		}
		doc, _ := jsonschema.UnmarshalJSON(bytes.NewReader(wire))
		if err := schema.Validate(doc); err != nil {
			t.Fatal(err)
		}
	}
	for _, refs := range []any{nil, []any{map[string]any{"path": "../escape", "digest": o["evidence_digest"]}}, []any{map[string]any{"path": "evidence.json", "digest": "bad"}}, []any{map[string]any{"path": "evidence.json", "digest": o["evidence_digest"]}, map[string]any{"path": "evidence.json", "digest": o["evidence_digest"]}}, []any{map[string]any{"path": "evidence.json", "digest": o["evidence_digest"], "extra": true}}} {
		o["evidence_references"] = refs
		_, err := DecodeResult(context.Background(), encode(t, old))
		errorCode(t, err, "invalid_input")
	}
	o["policy_decision"] = "pass"
	o["evidence_references"] = []any{}
	_, err = DecodeResult(context.Background(), encode(t, old))
	errorCode(t, err, "invalid_input")
	ref := `{"path":"evidence.json","digest":"` + o["evidence_digest"].(string) + `"}`
	wire := bytes.Replace(encode(t, old), []byte(`"evidence_references":[]`), []byte(`"evidence_references":[`+strings.Repeat(ref+",", MaxFiles)+ref+`]`), 1)
	_, err = DecodeResult(context.Background(), wire)
	errorCode(t, err, "limit_exceeded")
}
