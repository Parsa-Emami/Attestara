// Package licensecatalog shares the unchanged C02-pinned CycloneDX SPDX catalog.
package licensecatalog

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

//go:embed spdx.schema.json
var schema []byte

// Schema returns independent bytes for offline CycloneDX schema compilation.
func Schema() []byte { return bytes.Clone(schema) }

var catalog = sync.OnceValues(func() (map[string]string, error) {
	return loadCatalog(schema)
})

func loadCatalog(data []byte) (map[string]string, error) {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != "4b345e2329f209f34e960ae2a8e7cb46a166907e6a45e94978565925dc47b359" {
		return nil, errors.New("pinned SPDX catalog checksum mismatch")
	}
	var doc struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	// Exact sections verified against SPDX List 3.29.0 licenses/exceptions JSON;
	// hashes and provenance are retained with C13, not inferred from ID spelling.
	if len(doc.Enum) != 826 || doc.Enum[740] != "389-exception" {
		return nil, errors.New("unsupported SPDX catalog layout")
	}
	ids := make(map[string]string, len(doc.Enum))
	for i, id := range doc.Enum {
		kind := "license"
		if i >= 740 {
			kind = "exception"
		}
		ids[strings.ToLower(id)] = kind
	}
	return ids, nil
}

// Kind returns license, exception, or empty for an identifier absent from this
// pinned catalog. SPDX standard identifiers are case-insensitive.
func Kind(id string) (string, error) {
	ids, err := catalog()
	if err != nil {
		return "", err
	}
	return ids[strings.ToLower(id)], nil
}
