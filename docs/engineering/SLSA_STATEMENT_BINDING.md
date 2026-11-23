# Attestara 2.0 — SLSA statement subject binding

**Implemented scope:** A bounded local parser and CLI command for comparing in-toto Statement v1 / SLSA provenance v1 subject name/SHA-256 claims with a C04 release manifest and checked local file bytes.

```bash
go run ./cmd/attestara provenance \
  --input tests/fixtures/attestara/C30/manifest.json \
  --statement tests/fixtures/attestara/V2-01/slsa-provenance.json \
  --files tests/fixtures/attestara/C30
```

Run using the repository's supported Go toolchain. The CLI validates the existing C04 manifest, reads every manifest file through the confined root, verifies SHA-256 and declared sizes, validates CycloneDX, then compares SLSA statement subjects. The JSON output contains `schema_version`, `subject_bound`, `subjects_checked`, `file_bytes_checked` and `signer_trust` (always `unverified`). Mismatched claims exit 2.

**Security boundaries:** This feature does *not* verify a DSSE signature, Sigstore certificate, builder identity, source URI, SLSA level, run details, transparency inclusion or freshness. The predicate is an inert JSON object and its nested claims are not trusted. The standalone `internal/slsa` assessment has `file_bytes_checked=false` until the CLI performs actual confined file reads. The browser passport viewer remains read-only and inspection-only.

JSON processing rejects duplicate keys including nested predicates, malformed/invalid UTF-8 input, unsafe names, repeated subject names, unsupported versions, excessive recursion/token counts and payloads above 1 MiB. Multi-subject statements must match **all** listed files and include the manifest's designated subject.

Verification: `go test ./internal/slsa ./cmd/attestara ./tests` followed by `go test ./...` and `npm run release:check` on supported toolchain. This new code has no mandatory hosted or commercial dependency. DSSE verification and builder-policy acceptance are planned for V2-02/V2-03, not implemented.
