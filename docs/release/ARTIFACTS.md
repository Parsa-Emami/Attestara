# Complete local artifacts (C31)

The supported artifact is Linux x86_64, one local OS operator, profile
`local-cli-snapshots`. It includes the real Go engine, built web UI, offline
embedded schemas/catalog, example rules/inputs, licenses, SBOM and build provenance.
Node 24.19.0 is an external runtime prerequisite. Go and npm are needed to build,
not to run the binary distribution. OPA, Syft and Grype are not bundled or required.

## Build from a reviewed source snapshot

Install the pinned Node 24.19.0, npm 11.9.0, Go 1.27.2, Git, Python 3 and GNU
tar, gzip and coreutils prerequisites. The source-build route for Go is in
`docs/engineering/RESOURCE_LOCK.md`. On the supported Linux host:

```sh
npm run package
```

The command requires a clean Git checkout. It checks the resource lock and tool
versions, copies the committed source into private staging, installs the npm lock
with lifecycle scripts disabled, and builds fresh web/Go outputs there. It never
uses a prior `dist/web` or engine binary. Dependency preparation requires network
access or already populated package caches; offline **execution**, not air-gapped
source preparation, is supported. Only the reviewed repository's build is run;
selected user evidence/source bytes are never executed.

Outputs are under `dist/packages/<full-source-commit>/`:

- `attestara-<commit>-linux-x64.tar.gz`: complete runnable local product.
- `attestara-<commit>-source.tar.gz`: product sources, tests, locks, resources,
  documentation and build recipes; no Git metadata or dependency caches.
- `sbom.cdx.json`: CycloneDX 1.7 inventory, validated by the real embedded schema
  validator through the existing CLI. `attestara:distribution` distinguishes
  runtime components from build-only npm lock entries. Optional platform packages
  in the lock are not claims that their code was distributed in the runtime.
- `release.json`: unsigned source identity, tool versions, target and input hashes.
- `SHA256SUMS.txt`: hashes of all other top-level output files.

A destination is never overwritten, even if it is empty. Preserve earlier outputs;
choose another checkout for another candidate. A failed publication/parent flush
may have left a complete destination: inspect checksums before retrying. Killed
builds can leave `.c31-*` staging directories; inspect only task-owned leftovers
before manual cleanup. No accepted application store is read or copied by packaging.

## Run the binary distribution

Obtain the expected checksum manifest through a trusted channel, then:

```sh
cd dist/packages/<full-source-commit>
sha256sum -c SHA256SUMS.txt
mkdir /absolute/new-install-directory
tar -xzf attestara-<commit>-linux-x64.tar.gz -C /absolute/new-install-directory
cd /absolute/new-install-directory
sha256sum -c SHA256SUMS.txt
node scripts/start-local.mjs --store /absolute/private/attestara-store --port 4173
```

Use a fresh extraction directory; never extract over a store or another release.
Open `http://127.0.0.1:4173/#passport`. The CLI is `dist/engine/attestara`; follow
`docs/engineering/C30_LOCAL_WORKFLOW.md` using the included C30 synthetic fixtures.
Keep the store outside the release directory and private to its OS operator.
The service and CLI use immutable operations; restoring a backup creates a new
store only. Policy, file integrity and independently configured signer trust are
separate assessments. The viewer renders unverified local claims without uploads.

No database, login service, public domain or paid account is needed. HTTP domain
admission remains disabled. systemd/Nginx files are optional operator templates,
not an installed or authenticated hosting service; see WEB_RUNBOOK.md.

## Rebuild and reproduce

Extract the source archive in a fresh directory. `SOURCE_MANIFEST.json` records
exact included paths, SHA-256s, modes, original commit/time and explicit omissions.
The same `npm run package` verifies those recorded inputs without needing `.git`.
Optional copied agent skill directories without their own upstream LICENSE are
omitted from distribution only; the repository is untouched. These files are not
needed to build or run the product. Historical v7 sources are retained as inert
history; never run their bootstrap.

The source identity is SHA-256 of the deterministic JSON file inventory, including
paths, hashes and modes. Archives normalize ordering, owner/group and time to the
source commit. The engine uses CGO=0, trimpath and buildvcs=false. Reproducibility
means identical archives for the same source snapshot and pinned tools on the
supported profile; it does not promise byte equality across arbitrary toolchains.
`npm run test:package` tests two independent build paths and a source-archive rebuild,
then executes the packaged workflow, restart, restore and browser parity.

Checksums detect accidental modification against a separately trusted expected
manifest; neither checksums nor this unsigned provenance authenticate a publisher.
Independent release signatures/upgrade proof belong to C35; publication requires
C40 authorization. No artifact here is a published OSS v1 claim.

License texts for distributed components accompany `THIRD_PARTY_NOTICES.md` and
`license-inventory.json`. The Go runtime and schema licenses are retained under
`docs/engineering/resources/`; React/scheduler and Vite runtime-helper licenses
are under `licenses/npm/`. Build-only MPL-2.0 tools are referenced in the lock/SBOM;
their code is not bundled into the product executable or JS runtime as a library.

Build/format references: https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies
and https://reproducible-builds.org/docs/archives/. The npm dependency lock remains
the source of exact package versions/SRI; no runtime provider inventory is inferred.
