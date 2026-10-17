package licensecatalog

import "testing"

func TestCatalogRejectsMissingAndModifiedBytes(t *testing.T) {
	modified := Schema()
	modified[100] ^= 1
	for _, data := range [][]byte{nil, []byte("{}"), modified} {
		if got, err := loadCatalog(data); err == nil || got != nil {
			t.Fatal("missing or modified catalog accepted", err)
		}
	}
	if _, err := loadCatalog(Schema()); err != nil {
		t.Fatal("pinned catalog rejected", err)
	}
}
