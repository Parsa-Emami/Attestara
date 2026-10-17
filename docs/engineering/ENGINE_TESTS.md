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
