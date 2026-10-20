# Complete local artifacts (C31/C35)

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

First follow the detached-signature verification procedure below using a reviewed
copy of the verifier and independently established trust/expected source. Then:

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
The C35 verifier authenticates a detached signature over the checksum manifest,
using independently configured trust. The package command itself remains unsigned.
The dedicated maintainer public key and staged C40 publication procedure are in
[RELEASING.md](RELEASING.md); real publication requires separate authorization.
No artifact here is a published OSS v1 claim.

## Verify a detached release signature (C35)

Use `scripts/verify-release.mjs` from a previously reviewed source checkout, before
extracting or executing the download. Running the verifier from an unverified
archive cannot bootstrap trust. It uses only the pinned Node runtime and its
built-in crypto/filesystem APIs; no network service or Go/npm runtime is needed.

Obtain the expected full source commit and signer/public-key mapping through an
independently trusted channel. Do not copy trust from the download or assume its
embedded candidate key is trusted. The existing version-1 trust format is:

```json
{"schema_version":"1","trusted_signers":{"independently-established-signer":"64_HEX_CHARACTERS_FROM_AN_INDEPENDENT_CHANNEL"}}
```

The detached descriptor uses the existing C30 fields `schema_version: "1"`,
`signer_id`, `public_key_hex` (32-byte Ed25519 key) and `signature_hex` (64 bytes).
It signs the **exact bytes of the top-level SHA256SUMS.txt**, not reserialized JSON
or just release.json. All four C31 outputs must be listed exactly once and match:
the Linux runtime archive, source archive, SBOM and release metadata. Extra,
duplicate or unsafe checksum entries fail. Artifact files are read as regular
files without following symlinks, with streaming SHA-256 and a 1 GiB per-artifact
limit. The checksum/signature files are capped at 16 KiB and trust/release metadata
at 64 KiB. The supported metadata profile is `linux-x64`/`local-cli-snapshots`.

```sh
node /absolute/reviewed-source/scripts/verify-release.mjs --directory /absolute/download-directory --signature /absolute/release-signature.json --trust /absolute/independent-trust.json --expected-source-commit EXPECTED_FULL_SOURCE_COMMIT --freshness fresh
```

From a reviewed checkout, `npm run verify:release --` accepts the same arguments.
The JSON assessment keeps `file_integrity`, `cryptographic_valid`, `signer_trusted`,
`expected_source_match`, `freshness` and `release_trusted` separate. Exit 0 requires
all checks and explicit fresh classification. Exit 1 means unsigned, untrusted or
stale; 2 means invalid/missing/modified input or a different expected source;
3 means verification was operationally unavailable. Never install on a nonzero
exit. The optional absence of `--signature` cannot produce trusted success.

`fresh` is the operator's classification, not a clock-based expiry guarantee.
Once the operator expects a newer commit, an older signed package fails that
expectation. Rechecking an explicitly selected package is stateless. There is no
automatic latest-version discovery, persistent anti-replay authority or implicit
trust-on-first-use. A rollback explicitly selects the compatible earlier software;
it never restores older data over the current store. Verification is a point-in-time
assessment: keep download/install directories private and unchanged through use.

### Creating the detached descriptor

Signing happens separately, after the reproducible package has been completed.
An operator who already controls a reviewed Ed25519 PKCS#8 private key can use the
following recipe. Keep that key outside source/artifacts and establish its public
identity independently; the recipe does not enroll trust or select a production key.

```sh
node --input-type=module - /absolute/download-directory/SHA256SUMS.txt /private/ed25519-key.pem /absolute/new-signature.json independently-established-signer <<'NODE'
import { readFile, writeFile } from 'node:fs/promises';
import { createPrivateKey, createPublicKey, sign } from 'node:crypto';
const [checksums, privatePath, output, signer] = process.argv.slice(2);
const key = createPrivateKey(await readFile(privatePath));
if (key.asymmetricKeyType !== 'ed25519') throw new Error('Ed25519 key required');
const publicKey = createPublicKey(key).export({ format: 'jwk' });
const descriptor = { schema_version: '1', signer_id: signer,
  public_key_hex: Buffer.from(publicKey.x, 'base64url').toString('hex'),
  signature_hex: sign(null, await readFile(checksums), key).toString('hex') };
await writeFile(output, JSON.stringify(descriptor, null, 2) + '\n', { flag: 'wx' });
NODE
```

C35 tests generate ephemeral keys in memory. Those signatures demonstrate the
real mechanism on locally built artifacts; they are not the maintainer's identity
and are never a production trust root. Real publication remains C40.

## Upgrade and roll back without replacing accepted data

Keep releases in separate new directories and the private store outside them.
Before switching, export each operation you need to back up using the existing
CLI; one export contains one operation, not a whole-store snapshot. Verify the
new package as above, extract it into an unused directory, and check its internal
checksums. Leave the current release and store in place on any failure.

Stop the current launcher and start the new installed launcher with the **same
absolute --store path**. Re-read existing operations with its CLI before accepting
new ones. The current data formats are unchanged; no migration runs. If the new
installation is incomplete, continue using the previous complete installation.

For rollback, stop the new launcher and start the compatible earlier installation
against that same store. Verify both old and newly accepted operation IDs. Never
extract a release into the store, overwrite it with a backup, delete newer accepted
objects, or treat an old backup as ordinary software rollback. Existing destinations
are rejected by restore. Restore exports only into a fresh private directory and
verify them there. If an earlier reader rejects a newer format, retain the latest
bytes and compatible reader and repair forward; do not edit the version header.

`npm run test:package` also builds the pinned C34 baseline and installs it alongside
the candidate, verifies test signatures, writes distinct snapshots, restarts,
rolls software back and checks byte preservation, restore refusal/new-store recovery,
unsupported-format refusal and both browser viewers. Public test-key/signature and
artifact identities are retained under `dist/release-verification/`. See C35 evidence
for actual results; this same-host test does not certify C36 independent onboarding.

License texts for distributed components accompany `THIRD_PARTY_NOTICES.md` and
`license-inventory.json`. The Go runtime and schema licenses are retained under
`docs/engineering/resources/`; React/scheduler and Vite runtime-helper licenses
are under `licenses/npm/`. Build-only MPL-2.0 tools are referenced in the lock/SBOM;
their code is not bundled into the product executable or JS runtime as a library.

Build/format references: https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies
and https://reproducible-builds.org/docs/archives/. The npm dependency lock remains
the source of exact package versions/SRI; no runtime provider inventory is inferred.
