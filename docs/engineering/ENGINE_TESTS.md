# Engine verification (C06)

C06 verifies the implemented C03 process, C04 typed byte validators and C05 local file repository. The owner approved this N06 boundary. Cryptographic validity and signer trust remain F10/C16; full tampered/unsigned/untrusted/trusted bundle outcomes remain F20/C26. Revision expiry and authority replacement remain unimplemented. Storage operation replay is not revision authority.

## Toolchain and local commands

Use the Linux x86_64 Node 24.19.0/npm 11.9.0 and Go 1.27.2 identities in `toolchain-lock.json`. The C02 source-build route is documented in `RESOURCE_LOCK.md`. The initial Go 1.26.2 compiler only bootstraps the locked Go source; it cannot compile this module. Put the verified final Go compiler on command-local PATH and set `GOTOOLCHAIN=local` to prevent automatic toolchain substitution.

Run from the repository root:

```sh
node scripts/verify-engine-tests.mjs
go build ./...
go vet ./...
go test -count=1 -v ./...
```

The discovery command invokes `go test -list '^Test'` separately for each package below. A failed command, missing package or missing representative test fails the guard. Discovery is followed by actual uncached execution of every Go test through `./...`; listing alone certifies no behavior. The guard's missing/empty/no-test-output regressions run with `npm test`.

| Package | Real behavior exercised |
| --- | --- |
| `./internal/licensecatalog` | C13 missing/modified embedded catalog bytes fail closed |
| `./cmd/attestara` | Health contract, real listener, shutdown, occupied port, invalid launch and refused user source arguments |
| `./internal/contracts` | C04 immutable fixtures, strict JSON, missing evidence, versions, size/deadline/cancellation and inert source bytes |
| `./internal/storage` | C05 real files, independent process locking/SIGKILL recovery, injected ENOSPC, corruption and private inert bytes |
| `./tests` | C04/C05 storage integration; C10–C12 inert evidence adapters; C13 SPDX expression recognition, review outcomes, safe errors, shared schema pins and fresh-process repetition |

Existing Apache-2.0 repository-authored fixtures under `tests/fixtures/attestara/C04/` and `C05/` are reused unchanged. Their provenance and exact byte hashes are recorded in C06 evidence. No parallel C06 release fixture or production domain behavior is introduced. OPA, Syft and Grype are optional resources unused by these code paths; no provider execution is certified.

## Runtime CI

C13 adds a shared, immutable SPDX catalog and a Go expression recognizer. The
discovery guard requires the C13 tests in `./tests`; the full suite also reruns
C10 schema validation after its catalog resource is moved. Fresh-process
assessment repetition certifies deterministic read-only behavior, not recovery
of persisted license decisions. `recognized` is not a compliance verdict.

`.github/workflows/runtime.yml` runs on pull requests and pushes to main with read-only contents permission and commit-pinned actions. It follows the existing web workflow's C02 checksum-verified Go source-build route, checks the resulting version against the lock and uses `GOTOOLCHAIN=local` for verification. Each fresh checkout runs discovery, build, vet and all Go tests. The existing web, planning and source-integrity workflows retain their checks.

Local verification uses an isolated source checkout containing the candidate bytes, with fresh dependency/build outputs and uncached Go test results. The evidence identifies its source manifest and tool binaries. It records local commands separately from remote GitHub Actions execution; adding this workflow does not prove a remote run passed.

## Proof limits

C06 adds no HTTP domain admission, identity, signature trust, revision authority, policy execution or browser workflow. Existing storage crash tests use real SIGKILL, while ENOSPC is injected at actual file-operation boundaries; neither is physical power-loss or full-volume proof. The 2 vCPU/2 GiB target remains unmeasured until C28. Full clean-host workflow/restart/restore remains C30 and published installation remains C40.

## C14 predicate and result persistence

Mandatory discovery now includes C14 rule/fact byte evaluation, result-v2 schema
and v1 reader compatibility, C05 mixed-version reopen, and real v2 commit/recovery.
The storage package additionally requires v2 SIGKILL, injected ENOSPC/lost-ACK,
and on-disk quota rejection tests. Full `go test -count=1 -v ./...` executes them.
C14 does not activate the future HTTP API or OPA provider. First-party policy
facts and passing predicates do not establish producer authenticity.

## C15 optional real OPA verification

C15 adds required discovery for fixed-policy CLI evaluation, exact digest/size
bindings, strict facts, safe provider errors, environment isolation, output caps,
process timeout and inert source. Run the full suite with the checksum-verified
C02 OPA 1.21.2 binary selected explicitly:

```sh
ATTESTARA_TEST_OPA=/absolute/path/to/opa go test -count=1 -v ./...
```

Real-provider tests also accept a direct, nonsymlinked copy of the pinned `opa`
binary on PATH for developer convenience. If an explicit path and that binary
are absent, those tests explicitly skip: a green provider-free run cannot verify
C15. Runtime CI now downloads the C02-pinned OPA release artifact, checks its
SHA-256, and runs real-provider C15 regressions with an explicit path. A missing
or mismatched artifact fails the job. The provider-free suite can still skip
optional-provider tests; that run alone is not C15 acceptance. This workflow
change does not prove a remote Actions run passed. C15 historical evidence records
a separate actual OPA binary identity and execution. C05 reopen and fresh Go/OPA
process repetition prove the v2 outcome
stays bound to the same bytes, not new power-loss or full-workflow recovery.

## C16 signature boundary

`go test ./tests -run '^TestC16' -count=1 -v` runs actual Ed25519 verification
against independently generated synthetic signatures, exact C08 manifest bytes,
configured signer/key matching, hostile/unsigned inputs, cancellation, real C08
missing/modified file checks and inert source-like text. Discovery requires all
five C16 tests. It does not certify expiry/revision/replay authority, CLI/viewer
integration or the full reference workflow. No external provider is required.
