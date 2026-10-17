// Package adapters validates inert external evidence formats.
package adapters

import (
	"bytes"
	"embed"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"sync"

	"attestara/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// CycloneDX preserves the original document, including fields not yet projected
// into domain models. Schema validity does not establish signature or producer trust.
type CycloneDX struct {
	BOMFormat   string
	SpecVersion string
	Raw         []byte
}

//go:embed schemas/*.schema.json
var cyclonedxSchemas embed.FS

var cyclonedxSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	// Resolve only embedded resources; never fall back to filesystem or network.
	c.UseLoader(jsonschema.SchemeURLLoader{})
	for _, name := range []string{"bom-1.7.schema.json", "cryptography-defs.schema.json", "jsf-0.82.schema.json", "spdx.schema.json"} {
		b, err := cyclonedxSchemas.ReadFile("schemas/" + name)
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		if err := c.AddResource("http://cyclonedx.org/schema/"+name, doc); err != nil {
			return nil, err
		}
	}
	return c.Compile("http://cyclonedx.org/schema/bom-1.7.schema.json")
})

func cdxFailure(code, field, message string) error {
	return &contracts.Error{Code: code, Field: field, Message: message}
}

// DecodeCycloneDX validates CycloneDX JSON 1.7 against the C02-pinned official
// Draft 7 schemas. It performs no I/O, execution, trust verification or persistence.
// Callers must bound allocation before supplying bytes and not mutate them during decoding.
func DecodeCycloneDX(data []byte) (CycloneDX, error) {
	if int64(len(data)) > contracts.MaxBundleBytes {
		return CycloneDX{}, cdxFailure("limit_exceeded", "$", "JSON exceeds the 100 MiB byte limit.")
	}
	// JSON v2 rejects duplicate names and invalid UTF-8, including nested values.
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		var semantic *json.SemanticError
		if errors.As(err, &semantic) {
			return CycloneDX{}, cdxFailure("invalid_input", "$", "Expected a CycloneDX JSON object.")
		}
		return CycloneDX{}, cdxFailure("invalid_json", "$", "Expected one well-formed JSON object with unique member names and valid UTF-8.")
	}
	if fields == nil {
		return CycloneDX{}, cdxFailure("invalid_input", "$", "Expected a CycloneDX JSON object.")
	}
	var format, version string
	if err := json.Unmarshal(fields["bomFormat"], &format); err != nil || format != "CycloneDX" {
		return CycloneDX{}, cdxFailure("invalid_input", "bomFormat", "Expected CycloneDX format.")
	}
	if err := json.Unmarshal(fields["specVersion"], &version); err != nil || version == "" {
		return CycloneDX{}, cdxFailure("invalid_input", "specVersion", "A string specification version is required.")
	}
	if version != "1.7" {
		return CycloneDX{}, cdxFailure("unsupported_version", "specVersion", "Only CycloneDX JSON 1.7 is supported.")
	}
	schema, err := cyclonedxSchema()
	if err != nil {
		return CycloneDX{}, cdxFailure("dependency_unavailable", "$", "Pinned CycloneDX schema is unavailable.")
	}
	// The validator's decoder retains exact JSON numbers rather than float64.
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil || schema.Validate(doc) != nil {
		return CycloneDX{}, cdxFailure("invalid_input", "$", "Document does not conform to the pinned CycloneDX 1.7 schema.")
	}
	return CycloneDX{BOMFormat: format, SpecVersion: version, Raw: bytes.Clone(data)}, nil
}
