# Attestara

Open-source local release-evidence CLI and browser passport viewer, under [Apache-2.0](LICENSE).

Attestara validates release manifests, actual file bytes and CycloneDX 1.7 SBOMs,
evaluates the built-in evidence-presence rule, produces portable passports and
assesses Ed25519 signatures against independently configured trust. Immutable
local snapshots support restart, backup and restore into a new store.
Policy decisions, byte integrity and signer trust are separate assessments.

## Open the viewer

[Use the browser viewer](https://parsa-emami.github.io/Attestara/#passport).
Select a passport JSON file to inspect its claims without uploading it.
The viewer does not verify hashes, signatures or signer trust and has no accounts
or server-side evidence storage. Use the local CLI for those operations.

## Install and run locally

Supported profile: Linux x86_64, one local OS operator. Prerequisites:
Git, Node 24.19.0, npm 11.9.0 and Go 1.27.2 on PATH.
See [installation](docs/engineering/CLONE_TO_RUN.md) and the
[pinned Go source-build instructions](docs/engineering/RESOURCE_LOCK.md).

```sh
git clone https://github.com/Parsa-Emami/Attestara.git
cd Attestara
npm run setup
npm start
```

Open http://127.0.0.1:4173/#passport. Ctrl+C stops the application.
Choose another port with `npm start -- --port 4301`.
No database, Docker, domain or paid/cloud account is required.

[The user guide](docs/release/USER_GUIDE.md) walks through actual CLI evaluation,
verification, viewing, backup, restart and restore using licensed synthetic inputs.
[The CLI reference](docs/engineering/LOCAL_WORKFLOW.md) explains signature trust,
exit codes and immutable operation IDs.

## Downloads and builds

[Published v1.0.0](https://github.com/Parsa-Emami/Attestara/releases/tag/v1.0.0)
contains the signed Linux runtime and source archives for its tagged commit.
The `main` branch contains the current source. Older releases retain their original
contents; use a current source ZIP to obtain subsequent repository cleanup.

`npm run package` builds a fresh runtime and source distribution from a clean
committed checkout. [Artifact verification and installation](docs/release/ARTIFACTS.md)
describes independently trusted signatures, checksums and expected source identity.
The binary distribution needs Node; Go/npm are build tools.

## Development checks

```sh
npm run check
go test ./...
go vet ./...
npx --no-install playwright install chromium
npm run test:browser
```

[Testing](docs/engineering/ENGINE_TESTS.md) documents the optional OPA tests,
clean-clone workflow and complete release checks. CI retains reports as run
artifacts rather than committing generated execution logs into the source tree.

The supported rule is `evidence_present`; it evaluates a supplied boolean fact,
not a security scan. Freshness is explicitly classified by the operator.
Scanner execution, arbitrary policies, multi-user authorization, automatic expiry
and latest-release authority are unsupported. See [the security model](docs/security/THREAT_MODEL.md).

[Contributing](CONTRIBUTING.md) · [Security reports](SECURITY.md) ·
[Third-party notices](THIRD_PARTY_NOTICES.md) · [Maintenance](docs/release/MAINTENANCE.md).
