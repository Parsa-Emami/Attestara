# Attestara release-evidence contract v1

The executable C04 boundary is the Go library `attestara/internal/contracts`:

```go
input, err := contracts.DecodeInput(ctx, immutableJSONBytes)
result, err := contracts.DecodeResult(ctx, immutableResultBytes)
```

Both functions return a zero value and `*contracts.Error` on failure. No CLI,
HTTP admission, browser upload, provider execution or persistence is introduced.
The existing process health and unavailable domain routes retain their behavior.
The library accepts bytes, never opens manifest paths or executes source/tool text.

## Authority and compatibility

`input.schema.json` and `result.schema.json` describe executable version `"1"`.
All listed fields are required unless explicitly described below. Unknown fields,
duplicate object keys (including escaped aliases), case-altered keys, invalid UTF-8,
null values, trailing JSON and incorrect types are rejected. Text is nonblank and
at most 4096 Unicode code points, retaining the earlier draft's text bound.
Digests use `sha256:` plus exactly 64 lowercase hexadecimal characters, matching
C01. Hashes identify bytes; they do not establish producer trust.

The original draft schemas are preserved byte-for-byte in `drafts/*.v0.schema.json`.
Draft 0 was never a supported public format. It and unknown versions are rejected;
there is no automatic migration, prefix normalization or silent data dropping.
C01's specification fixtures remain historical and unchanged. Re-author v1 from
actual artifacts and preserve old exports before accepting new state. Future
incompatible changes require a new version and compatibility fixtures.
The existing OpenAPI/AsyncAPI files remain future transport designs and are not
activated by these file contracts. ADR-0003 and the approved library-only C04
scope supersede the earlier README's CLI-only delivery description.

## Input

The domain fields remain `subject_digest`, `source_revision`, `tool`,
`tool_version`, `evidence_digest`, `policy_digest` and `verification`.
`schema_version`, `release_id` and `owner` identify the version and C01 synthetic
release. `owner` is a descriptive local label, never authorization proof.
`verification` must be `unverified`: the caller cannot supply trusted authority.

`files` is the blueprint's concrete manifest: each entry has `path`, `digest`,
`size_bytes` and `media_type`. Paths are unique portable relative paths composed
of slash-separated ASCII segments beginning with a letter, digit, `_` or `-`;
remaining segment characters may also contain `.`. Absolute paths, dot segments,
backslashes, colons and URLs are rejected. No path is resolved by this library.
All three root digests must occur in the file manifest. Empty files are allowed;
missing/null/negative sizes are rejected. Sizes use plain integer JSON tokens
(without decimal points or exponents), matching the typed Go reader; this lexical
restriction is an additional reader rule beyond JSON Schema integer semantics. Actual file-byte hashing and ownership
checks belong to later adapters, not to metadata validation.

The parser counts files before allocating the typed manifest slice.
Product limits from `product/project.json` are 2000 files and 100 MiB total declared
file bytes, inclusive. Each input/result JSON byte slice also has a 100 MiB cap.
Aggregate size, unique paths and digest membership are Go invariants documented
in schema comments; JSON Schema alone cannot express these cross-field rules.
The two 100 MiB bounds apply separately, not as a combined heap budget.
Minimum-memory profile measurement remains C28 work.

## Results and errors

The envelope retains `schema_version`, `status`, `input_digest`, `engine_version`
and `warnings`. `input_digest` hashes the exact input JSON bytes, including
whitespace; there is no canonicalization. The decoder validates its shape, not
its association with a separately supplied input. Consumers must check that
binding against their input bytes before treating a result as associated.

For `complete`, `partial` or `unverified`, `result` is required and `error` is absent.
The typed result contains subject/evidence/policy digests, `policy_decision`
(`allow`, `deny`, `unknown`) and `verification` (`unverified`, `rejected`).
`complete` requires a known policy decision; `unverified` requires unverified
authenticity. Policy allow never implies authenticity. `trusted` is deliberately
withheld until cryptographic verification and signer trust have a proven contract.
Decoding an allow/deny value does not claim this library evaluated a policy.

For `failed`, `error` is required and `result` is absent. Errors contain nonblank
`code`, `field` and `message`. `$` denotes the whole document. Decoder errors use
safe messages and fixed field names without echoing source bytes or secrets.

| Code | Meaning |
| --- | --- |
| `invalid_json` | Syntax, duplicate keys, trailing JSON or invalid UTF-8 |
| `invalid_input` | Missing/unknown/null/wrong-type fields or domain invariant failure |
| `unsupported_version` | Explicit version other than `"1"` |
| `limit_exceeded` | Wire bytes, aggregate file bytes or file count exceeded |
| `cancelled` | Caller cancelled validation |
| `deadline_exceeded` | Caller or validation deadline elapsed |
| `input_unavailable` | A producer could not read required input; representable failure, not emitted by the byte decoder |
| `dependency_unavailable` | A producer lacks a required provider; representable failure, not a fabricated provider run |

Each decode uses the earlier caller deadline or the product's 10 second ceiling.
Cancellation is cooperative: checked before/after bounded JSON decoding and during
manifest traversal. It cannot preempt a standard-library JSON decode mid-call.
No blocking file/network I/O or detached timeout goroutine is involved. Future
policy execution must enforce its own 10 second deadline at its provider boundary.

## Reproduce

Use the pinned C02/C03 Node/npm/Go toolchain:

```sh
node --test tests/contracts/domain-contracts.test.mjs
go test ./...
go vet ./...
npm run check
```

`npm test` discovers the Node contract entry, which exercises the real Go validators.
It also runs fixture assertions in fresh Go test processes to prove repeatable
validation after process restart; this is not storage durability or restore proof.
The C04 corpus is repository-authored Apache-2.0 synthetic data. It includes a
concrete subject, CycloneDX 1.7 SBOM matching the C02 format pin and inert Rego v1
policy text. C04 needs no OPA/Syft/Grype process. The policy decision fixture remains
`unknown` and authenticity remains `unverified`.

Implementation references: [Go 1.27 JSON v2](https://pkg.go.dev/encoding/json/v2),
[context.WithTimeout](https://pkg.go.dev/context#WithTimeout),
[JSON Schema 2020-12 validation](https://json-schema.org/draft/2020-12/json-schema-validation).
CycloneDX source identity/license are retained in `docs/engineering/toolchain-lock.json`.
