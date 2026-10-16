> Current OSS delivery/resources/storage/release policy is ADR-0003-open-source-local-first.md and DEFINITION_OF_DONE.md. The older planning context below is superseded where it conflicts; business invariants remain applicable.

# Current web entry point

The browser shell is implemented at apps/web/src/main.tsx, with root package.json/package-lock.json and apps/web/vite.config.ts. Read ADR-0002-web-application.md and WEB_RUNBOOK.md for runnable web commands. The domain blueprint below describes remaining engine work, not completed web status.

# Attestara implementation blueprint v0

Planning detail; the file paths, commands and endpoints below are future targets, not installed capabilities.

## Executable proof and concrete layout

Entry point: `cmd/attestara/main.go`.

Module map: `internal/{bundle,evidence,adapters,policy,verify,decision,report,export}; apps/web/src/pages/Passport.tsx`.

First vertical slice: Load valid sample SBOM with manifest -> evaluate local policy -> produce passport -> change one byte -> verify failure -> distinguish unsigned from signed/trusted.

For Go repositories use a single root module; add cmd/agent, cmd/sensor or cmd/verify only where selected. For web assets add separate apps/web package.json/package-lock.json at C03; domain engine remains outside page components. Rust workspace and Python package names follow the paths in the commit plan. Versions must be verified/pinned at C02. No inferred 'latest' install command in documentation constitutes a lockfile.

## Persistence model

No DB for verifier/viewer. Bundle manifest maps relative files to digest/size/type. Optional future private scan mode needs jobs(subject_digest,id,state), idempotency, audit in SQLite; that mode is not implicitly enabled.

Plan constraints with actual types at C05: opaque ID, owner/scope where relevant, schema version, optimistic entity version, UTC creation/update times. Use unique indexes for idempotency and stable identity, foreign/check constraints where supported. Add indexes only for measured UI/engine queries. Where no DB is selected, explicitly test portable file integrity and failed write/import behavior instead of adding migrations.

## State and transport

RECEIVED -> VALIDATED -> VERIFIED|UNVERIFIED|FAILED -> POLICY_PASS|POLICY_FAIL|UNKNOWN -> EXPORTED.

Planned CLI verify/evaluate/export on local files. Viewer loads passport through browser file picker. No arbitrary source execution or hosted URL fetching.

Status queries are authoritative on actual operation state. Async 202 means accepted durable operation with status reference, not completed success. Errors distinguish validation, authorization, stale write, quota, cancellation, unavailable dependency and corrupted data. C04 replaces v0 generic nested schemas with typed domain shapes and bounded fixtures before first public format.

## Authentication and authorization

Local OS access and configured trusted signer identities/keys. A key embedded in an untrusted uploaded bundle does not make its signature trusted.

If an embedded API is enabled, identity must be established before commands and resource scope checked in the application layer; put foundational identity adapter/permission tests in C03/C06 and complete object-level negative cases alongside each API feature. Don't postpone all auth to C27. Cookie-auth mutations require CSRF; non-browser agent paths have a distinct authenticated channel. Static viewers do not need a user account just to read a local report.

## Configuration without env edits

Read a validated runtime JSON/config path plus platform-injected secret references. Precedence: explicitly passed config -> documented project config -> safe development defaults. Production rejects missing required trust/auth/durable path values. Do not silently invent production secrets. Common typed settings: mode, listen address, input limits, data directory (where needed), task deadline, retention, public-demo switch, log level. Existing input env examples remain archival.

Secrets: secret reference identifies independently injected credential/key; never real value in JSON. Demo uses synthetic fixtures and has dangerous write/runtime admission disabled. Target quotas in product/project.json must be validated; negative/zero TTL and unlimited quotas are rejected where safety-sensitive.

## Development verification workflow

At C03 add real developer commands for build/dev and tests for this toolchain. At C06 run them in clean CI. Go: go test ./... and go vet ./...; Rust: cargo test --workspace and cargo clippy --workspace; Python: actual pytest suite plus declared formatter/type checks; TS: typecheck/build and domain/UI tests with chosen test dependencies pinned. These are planned commands until code/manifests exist. Do not report them as executed in v0.

For each feature run its named target test plus relevant existing regression suite. Use real selected storage/provider in integration tests; no mock-only evidence for durability, authorization, cryptography or isolation. Fuzz hostile parser inputs, inject interrupt/failure and measure overload at release gates. Example fixture data must be synthetic or explicitly authorized.

## Clean install and recovery

Deliver single versioned CLI/static/container artifact appropriate to selected mode. Stateful profile mounts local persistent disk and runs one controlled owner. Fresh install proves first slice, failure behavior and export/recovery; update fixtures prove previous exported schema can be read or migrated without silent loss. Maintenance window is acceptable for personal single-host mode. Restore and forward repair are documented with real measured evidence before sensitive use.

Full release policy and hardware targets are in sibling docs. Native mobile-store packaging and shared multi-user cloud hosting are not implicitly part of this blueprint.
