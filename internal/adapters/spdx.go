package adapters

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"attestara/internal/contracts"
)

const spdxContext = "https://spdx.org/rdf/3.0.0/spdx-context.jsonld"

// NormalizedBOM is the C11 package/dependency subset of CycloneDX 1.7.
// A missing dependency entry means unknown dependencies, never dependency-free.
type NormalizedBOM struct {
	BOMFormat    string          `json:"bomFormat"`
	SpecVersion  string          `json:"specVersion"`
	Components   []BOMComponent  `json:"components"`
	Dependencies []BOMDependency `json:"dependencies"`
}
type BOMComponent struct {
	Type    string `json:"type"`
	Ref     string `json:"bom-ref"`
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	PURL    string `json:"purl,omitempty"`
}
type BOMDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn"`
}

// SPDX retains inert source metadata privately for subset export. Neither a
// projected BOM nor syntactic validity establishes authenticity or authority.
type SPDX struct {
	BOM    NormalizedBOM
	source []byte
}

type spdxDocument struct {
	Context string                      `json:"@context"`
	Graph   []map[string]jsontext.Value `json:"@graph"`
}

var spdxPurposes = map[string]string{
	"application": "application", "framework": "framework", "library": "library",
	"container": "container", "operatingSystem": "operating-system", "device": "device",
	"firmware": "firmware", "file": "file", "data": "data", "model": "machine-learning-model",
	"platform": "platform", "deviceDriver": "device-driver",
}

func spdxFailure(code, field string) error {
	return &contracts.Error{Code: code, Field: field, Message: "SPDX input is outside the documented C11 package/dependency subset."}
}
func spdxString(n map[string]jsontext.Value, key string) string {
	var s string
	if json.Unmarshal(n[key], &s) != nil {
		return ""
	}
	return s
}
func spdxStrings(n map[string]jsontext.Value, key string) ([]string, bool) {
	var s []string
	if json.Unmarshal(n[key], &s) != nil || len(s) == 0 {
		return nil, false
	}
	for _, v := range s {
		if v == "" {
			return nil, false
		}
	}
	return s, true
}
func spdxID(id string) bool {
	u, err := url.Parse(id)
	return err == nil && u.IsAbs() && !strings.ContainsAny(id, " \t\r\n\x00")
}

// DecodeSPDX accepts only the pinned 3.0.0 compact JSON-LD context, without
// context expansion or resource fetching. This is subset validation, not full
// SPDX schema/OWL conformance. Callers must bound allocation before passing bytes.
func DecodeSPDX(data []byte) (SPDX, error) {
	fail := func(code, field string) (SPDX, error) { return SPDX{}, spdxFailure(code, field) }
	if int64(len(data)) > contracts.MaxBundleBytes {
		return fail("limit_exceeded", "$")
	}
	// Validate syntax separately so malformed/duplicate/UTF-8 errors stay typed.
	if !jsontext.Value(data).IsValid() {
		return fail("invalid_json", "$")
	}
	var root map[string]jsontext.Value
	if json.Unmarshal(data, &root) != nil || root == nil || root["@context"] == nil || root["@graph"] == nil {
		return fail("invalid_input", "$")
	}
	var d spdxDocument
	if json.Unmarshal(data, &d) != nil {
		return fail("invalid_input", "$")
	}
	if d.Context != spdxContext {
		return fail("unsupported_version", "@context")
	}
	if len(root) != 2 || len(d.Graph) == 0 {
		return fail("invalid_input", "@graph")
	}
	ids := map[string]string{}
	creations := map[string]bool{}
	packages := map[string]bool{}
	bom := NormalizedBOM{BOMFormat: "CycloneDX", SpecVersion: "1.7", Components: []BOMComponent{}, Dependencies: []BOMDependency{}}
	documents := 0
	for _, n := range d.Graph {
		kind := spdxString(n, "type")
		if _, ok := n["@context"]; ok {
			return fail("invalid_input", "@context")
		}
		if _, ok := n["@type"]; ok {
			return fail("invalid_input", "type")
		}
		if _, ok := n["@id"]; ok && kind != "CreationInfo" {
			return fail("invalid_input", "@id")
		}
		id := spdxString(n, "spdxId")
		if kind == "CreationInfo" {
			id = spdxString(n, "@id")
			if _, ok := n["spdxId"]; ok {
				return fail("invalid_input", "@graph")
			}
			if !spdxID(id) && !(strings.HasPrefix(id, "_:") && len(id) > 2 && !strings.ContainsAny(id, " \t\r\n\x00")) {
				return fail("invalid_input", "@graph")
			}
		}
		if id == "" || ids[id] != "" || (kind != "CreationInfo" && !spdxID(id)) {
			return fail("invalid_input", "@graph")
		}
		ids[id] = kind
		switch kind {
		case "CreationInfo":
			if spdxString(n, "specVersion") != "3.0.0" {
				return fail("unsupported_version", "specVersion")
			}
			if _, err := time.Parse(time.RFC3339, spdxString(n, "created")); err != nil {
				return fail("invalid_input", "@graph")
			}
			creations[id] = true
		case "Person", "Organization", "Tool":
			if spdxString(n, "name") == "" {
				return fail("invalid_input", "@graph")
			}
		case "SpdxDocument":
			documents++
		case "software_Package":
			purpose := spdxString(n, "software_primaryPurpose")
			typ := spdxPurposes[purpose]
			if typ == "" {
				return fail("invalid_input", "software_primaryPurpose")
			}
			name := spdxString(n, "name")
			if name == "" {
				return fail("invalid_input", "@graph")
			}
			c := BOMComponent{Type: typ, Ref: id, Name: name}
			for key, dst := range map[string]*string{"software_packageVersion": &c.Version, "software_packageUrl": &c.PURL} {
				if value, ok := n[key]; ok {
					if json.Unmarshal(value, dst) != nil || *dst == "" {
						return fail("invalid_input", "@graph")
					}
				}
			}
			if c.PURL != "" && (!strings.HasPrefix(c.PURL, "pkg:") || !spdxID(c.PURL)) {
				return fail("invalid_input", "@graph")
			}
			packages[id] = true
			bom.Components = append(bom.Components, c)
		case "Relationship":
			if spdxString(n, "relationshipType") != "dependsOn" {
				return fail("invalid_input", "relationshipType")
			}
			for _, key := range []string{"completeness", "startTime", "endTime"} {
				if _, ok := n[key]; ok {
					return fail("invalid_input", key)
				}
			}
		default:
			return fail("invalid_input", "type")
		}
	}
	if documents != 1 || len(packages) == 0 || len(creations) == 0 {
		return fail("invalid_input", "@graph")
	}
	edges := map[string]map[string]bool{}
	for _, n := range d.Graph {
		kind := spdxString(n, "type")
		if kind == "CreationInfo" {
			creators, ok := spdxStrings(n, "createdBy")
			if !ok {
				return fail("invalid_input", "@graph")
			}
			for _, id := range creators {
				if ids[id] != "Person" && ids[id] != "Organization" && ids[id] != "Tool" {
					return fail("invalid_input", "@graph")
				}
			}
			continue
		}
		if !creations[spdxString(n, "creationInfo")] {
			return fail("invalid_input", "@graph")
		}
		if kind == "SpdxDocument" {
			roots, ok := spdxStrings(n, "rootElement")
			if !ok {
				return fail("invalid_input", "@graph")
			}
			for _, id := range roots {
				if !packages[id] {
					return fail("invalid_input", "@graph")
				}
			}
		}
		if kind != "Relationship" {
			continue
		}
		from := spdxString(n, "from")
		to, ok := spdxStrings(n, "to")
		if !packages[from] || !ok {
			return fail("invalid_input", "@graph")
		}
		if edges[from] == nil {
			edges[from] = map[string]bool{}
		}
		for _, id := range to {
			if !packages[id] {
				return fail("invalid_input", "@graph")
			}
			edges[from][id] = true
		}
	}
	sort.Slice(bom.Components, func(i, j int) bool { return bom.Components[i].Ref < bom.Components[j].Ref })
	for from, targets := range edges {
		dep := BOMDependency{Ref: from, DependsOn: []string{}}
		for id := range targets {
			dep.DependsOn = append(dep.DependsOn, id)
		}
		sort.Strings(dep.DependsOn)
		bom.Dependencies = append(bom.Dependencies, dep)
	}
	sort.Slice(bom.Dependencies, func(i, j int) bool { return bom.Dependencies[i].Ref < bom.Dependencies[j].Ref })
	return SPDX{BOM: bom, source: bytes.Clone(data)}, nil
}

// EncodeSPDX serializes a decoded projection back to the supported SPDX subset.
// It retains source metadata and package identities; structural editing is not
// supported. Multiple dependency assertions with one source are grouped on export.
func EncodeSPDX(v SPDX) ([]byte, error) {
	invalid := func() ([]byte, error) { return nil, spdxFailure("invalid_input", "projection") }
	original, err := DecodeSPDX(v.source)
	if err != nil {
		return invalid()
	}
	// Read-only import/export supports package fact edits, not graph editing.
	if v.BOM.BOMFormat != "CycloneDX" || v.BOM.SpecVersion != "1.7" || len(v.BOM.Components) != len(original.BOM.Components) || !reflect.DeepEqual(v.BOM.Dependencies, original.BOM.Dependencies) {
		return invalid()
	}
	components := map[string]BOMComponent{}
	for i, c := range v.BOM.Components {
		if c.Ref != original.BOM.Components[i].Ref || c.Name == "" {
			return invalid()
		}
		components[c.Ref] = c
	}
	normalized, err := json.Marshal(v.BOM)
	if err != nil {
		return invalid()
	}
	if _, err = DecodeCycloneDX(normalized); err != nil {
		return invalid()
	}
	var d spdxDocument
	if json.Unmarshal(v.source, &d) != nil {
		return invalid()
	}
	set := func(n map[string]jsontext.Value, k string, value any) { b, _ := json.Marshal(value); n[k] = b }
	dependencies := map[string][]string{}
	for _, dep := range v.BOM.Dependencies {
		dependencies[dep.Ref] = dep.DependsOn
	}
	graph := make([]map[string]jsontext.Value, 0, len(d.Graph))
	seen := map[string]bool{}
	for _, n := range d.Graph {
		switch spdxString(n, "type") {
		case "software_Package":
			c := components[spdxString(n, "spdxId")]
			purpose := ""
			for p, t := range spdxPurposes {
				if t == c.Type {
					purpose = p
				}
			}
			if purpose == "" {
				return invalid()
			}
			set(n, "spdxId", c.Ref)
			set(n, "name", c.Name)
			set(n, "software_primaryPurpose", purpose)
			for k, value := range map[string]string{"software_packageVersion": c.Version, "software_packageUrl": c.PURL} {
				delete(n, k)
				if value != "" {
					set(n, k, value)
				}
			}
		case "Relationship":
			from := spdxString(n, "from")
			if seen[from] {
				continue
			}
			seen[from] = true
			set(n, "from", from)
			set(n, "to", dependencies[from])
		}
		graph = append(graph, n)
	}
	d.Graph = graph
	b, err := json.Marshal(d)
	if err != nil {
		return invalid()
	}
	if _, err = DecodeSPDX(b); err != nil {
		return invalid()
	}
	return b, nil
}
