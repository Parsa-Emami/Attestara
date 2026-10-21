# Build and test

Use the pinned Linux x86_64 Node/npm/Go toolchain from
[RESOURCE_LOCK.md](RESOURCE_LOCK.md).

```sh
npm run setup
npm run check
go build ./...
go vet ./...
go test -count=1 -v ./...
```

Go tests cover strict file contracts, real file/ZIP hashing, SBOM parsing, policy
results and reasons, signature trust, passport binding, source-input inertness,
immutable snapshots, conflicts and recovery. `scripts/verify-engine-tests.mjs`
checks discovery of the required behavioral tests. Tests and their licensed
fixtures remain part of the source; generated results belong under ignored `dist/`.

## Optional real OPA

Provide the checksum-verified OPA artifact from the resource lock:

```sh
ATTESTARA_TEST_OPA=/absolute/path/to/opa go test -count=1 -v ./...
```

Without it, optional real-provider cases explicitly skip. A provider-free test
run does not verify OPA execution. Runtime CI downloads and verifies the pinned
binary and runs those cases explicitly. The supported core CLI does not invoke OPA.

## Browser, guide and fresh installation

```sh
npx --no-install playwright install chromium
npm run test:browser
node --test tests/ops/user-guide.test.mjs
npm run test:clone
npm run test:package
```

Browser tests use the real supervised application and cover passport import,
keyboard/narrow-screen use, invalid/cancelled reads, engine outages and inert
hostile inputs. `WEB_TEST_CHROMIUM` may select an explicitly installed compatible
Chromium executable. Nginx is needed for `tests/ops/nginx-local.test.mjs`.

The clean-clone test uses a fresh Git transport clone and isolated Linux runtime:
`bwrap` locally, or an ephemeral GitHub Actions VM in CI. It exercises setup, the
literal public-guide commands, CLI negatives/replay/conflict, process restart,
new-store restore and the browser viewer. It requires at least 10 GiB initially
free disk. Browser installation requires network unless a system test browser is
explicitly selected. Optional scanners/providers are not part of this workflow.

The package test verifies actual installed artifacts, source rebuild reproducibility,
signatures/checksums and compatible upgrade/rollback retaining newer store data.
Its fixed historical baseline is fetched from Git history; no historical roadmap
or generated evidence files are required in the current checkout.

For the complete current-source release gate, see [RELEASE_GATE.md](RELEASE_GATE.md).
