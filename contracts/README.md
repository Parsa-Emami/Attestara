# File contracts

Attestara uses versioned local JSON files. HTTP/event API drafts are not part of
the running application. The Go validators are in `internal/contracts`:
`DecodeInput` and `DecodeResult` reject malformed JSON, duplicate/unknown fields,
invalid UTF-8, unsupported versions and inconsistent required values.

## Input v1

`input.schema.json` describes `schema_version`, `release_id`, `owner`,
`subject_digest`, `source_revision`, `tool`, `tool_version`, `evidence_digest`,
`policy_digest`, `verification` and `files`.

The caller must supply `verification: unverified`. `owner` is a descriptive label,
not authorization. Digests are `sha256:` plus 64 lowercase hexadecimal characters.
Each file declares a unique portable relative path, digest, nonnegative integer
`size_bytes` and media type. All three root digests must appear in the manifest.
Absolute paths, traversal, backslashes and URLs are rejected. Admission is bounded
to 2000 files and 100 MiB of declared file bytes; input/result JSON has its own
100 MiB bound. Cross-field invariants are also enforced in Go.

The parser validates metadata only. Full `evaluate --files` and `verify` operations
read actual referenced bytes, compare size/hashes and confine paths. Full evaluation
requires at least one schema-valid CycloneDX JSON 1.7 SBOM.

## Results v1/v2 and passports

`result.schema.json` retains result v1 compatibility. `result.v2.schema.json` adds
`pass/fail/unknown` and bound evidence references. Existing result v1/v2 data can
coexist. Unknown versions and the old v0 drafts are rejected; there is no automatic
migration or implicit trust upgrade. Draft schemas are retained solely as backward
rejection test inputs.

Result `input_digest` binds the exact input JSON bytes, including whitespace.
Root evidence/policy digests bind their exact file bytes. The built-in rule is
`{"schema_version":"1","predicate":"evidence_present"}` and consumes a supplied
versioned boolean fact. True produces pass, false fail, missing/null unknown;
explicit stale classification produces unknown. It does not scan software or infer
license compliance, complete SBOM coverage or trusted producer identity.

A passport contains release/source IDs, subject/evidence/policy digests, decision,
references, warnings and canonical `report_digest`. Its digest covers every field
except itself. The CLI verifies that digest; the browser only displays claims.
Independent Ed25519 verification signs the exact manifest bytes and requires
separately configured signer/key trust. Policy pass never establishes authenticity.

See [the CLI reference](../docs/engineering/LOCAL_WORKFLOW.md) and
[user guide](../docs/release/USER_GUIDE.md). Run `npm test` and `go test ./...` for
real parser, byte-binding, signature, storage and recovery regressions.
