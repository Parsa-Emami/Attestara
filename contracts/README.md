# Attestara release-evidence contracts

The executable C04 boundary (extended by C14 result v2) is the Go library `attestara/internal/contracts`:

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

## C14 result v2 and local policy evaluation

`result.schema.json` remains the unchanged v1 schema. `result.v2.schema.json`
describes result version `"2"`; input remains version `"1"`. `DecodeResult`
reads both versions. The v1 decoder still rejects v2 fields, including an explicit
null/empty `evidence_references`, and retains `allow/deny/unknown`. Historical
C04 fixture `result-version-2` now fails as an incomplete v2 payload; a separate
regression rejects version 3. No existing stored result or fixture is rewritten.

V2 retains the envelope, digest bindings and separate authenticity status. Its
`policy_decision` is `pass`, `fail` or `unknown`; `policy_digest` hashes the exact
rule JSON bytes, including whitespace. `evidence_references` is a required array
of `{path, digest}` objects, capped at 2000 unique paths. Known decisions require
at least one reference. The C05 adapter checks references against the associated
input's root evidence artifact and checks actual file bytes before commit and
on reopen. The store manifest format and operation IDs are unchanged; v1 and v2
result payloads can coexist. Older software that only supports result v1 cannot
read v2 payloads: preserve exports and use the compatible reader, not an old binary
against newer accepted state. There is no automatic data migration.

The internal Go entry point is:

```go
result, err := policy.Evaluate(ctx, inputJSON, ruleJSON, evidenceJSON)
```

The only supported rule is `{"schema_version":"1","predicate":"evidence_present"}`
(with insignificant whitespace permitted but hashed exactly). Rule bytes are
capped at 4096. Evidence is a first-party JSON fact document with version `"1"`
and optional `evidence_present` boolean. True yields pass; explicit false yields
fail; absent/null fact or unavailable evidence yields unknown. Actual evidence
bytes must match the input evidence digest and manifest size before a known
decision is returned. Nil evidence bytes mean unavailable; non-nil empty bytes
are parsed and rejected as malformed JSON. A missing evidence file cannot be
persisted through C05 even if an in-memory unknown result was produced.

Rules and facts reject unknown/duplicate members, malformed UTF-8/JSON and
unsupported versions with safe `contracts.Error` messages and empty results.
Evidence/input use C04's 100 MiB boundary. The caller deadline or 10 second cap
is checked cooperatively; synchronous JSON parsing and hashing are not preempted.
References retain manifest order and returned slices are newly owned.

This predicate evaluates the supplied first-party fact; it does not infer scanner
coverage, SBOM correctness, license compliance, producer identity or signature
trust. All generated outcomes retain unverified authenticity, including pass.
No rule source, Rego, shell, URL, scanner or provider is executed or fetched.
The HTTP/AsyncAPI designs remain inactive. OPA execution belongs to C15.

Implementation follows the existing [Go JSON v2 decoder](https://pkg.go.dev/encoding/json/v2#Unmarshal)
and [context deadline](https://pkg.go.dev/context#WithTimeout) boundaries.

## C15 bounded local OPA adapter

```go
result, err := adapters.EvaluateOPA(ctx, absoluteOPAPath, inputJSON, policyBytes, evidenceJSON)
```

The internal adapter requires the owner-selected OPA 1.21.2 executable and the
exact existing C04 Rego policy (SHA-256
`52f2452badde5b273ce44fb4663e08868116d8a1151544048021d8634b83483e`).
It is deliberately limited to that policy and the C14 version-1 fact document;
arbitrary Rego, scanner-specific fields and caller-supplied queries are unsupported.
Input remains contract v1; output remains result v2. Policy digest binds the exact
Rego bytes here; C14 continues to bind its rule JSON. References bind actual fact
bytes and manifest sizes. Hash binding and policy pass do not establish authenticity.

The fixed query `data.attestara.release.allow` returns a boolean: true becomes
pass and false becomes fail. The C04 policy defaults to false for an absent/null
fact; C14's first-party predicate still returns unknown in that case. Unavailable
evidence bytes fail with `input_unavailable`. An undefined/non-boolean provider
result fails; it is never converted to pass or unknown.

The provider receives facts via stdin with an empty environment, fixed argv and
no shell. Only policy bytes are written into a private temporary directory and
removed after evaluation. Policy, stdout and stderr each have a 4096-byte cap;
input/facts use the existing 100 MiB limit. The caller deadline or shared 10-second
cap bounds version probing and evaluation together. OPA's own evaluation timeout
is also enabled. Timeout returns an empty result and `deadline_exceeded`; explicit
cancellation returns `cancelled`; missing/wrong-version providers return
`dependency_unavailable`. Malformed outputs, failed policy evaluation and output
limits fail with existing typed errors and input-independent messages.

The executable path is trusted application configuration, never upload data.
Version checking is compatibility checking, not executable authenticity or a
sandbox for hostile binaries. No provider is installed automatically. Subject
bytes, shell fragments and uploaded source are never executed. The HTTP API and
browser domain workflow remain future work. CLI flags follow the
[official OPA reference](https://www.openpolicyagent.org/docs/cli#eval).
