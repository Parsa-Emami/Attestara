# Maintainer release: v1.0.0 (C40)

Target: Linux x86_64, one OS operator, `local-cli-snapshots`. The release tag is
`v1.0.0`; archive names and provenance keep the exact source commit. Component
health/web version `0.0.1` is unchanged. A release tag does not change the health
contract or grant additional platform, provider, identity or capacity support.

## Trust and authorization

`RELEASE_MANIFEST.json` pins the release version/profile and independently reviewed
Ed25519 public key. Compare its fingerprint with the maintainer through a trusted
channel before configuring trust. A key accompanying an untrusted download cannot
bootstrap trust. Signatures cover the exact top-level `SHA256SUMS.txt` bytes and
use the existing C35 descriptor, including signer ID and raw public key.

The first dedicated key was created outside Git at
`~/.local/share/attestara/release-signing/v1/private.pem` (directory 0700, file 0600).
It must be backed up privately by the maintainer. Never copy it into the repository,
chat, build artifacts or logs. Do not recreate or silently replace a missing key.
The checked-in public fingerprint is the release identity; synthetic C35 keys are
only test fixtures and are never that identity.

Before real publication, the repository maintainer must configure the GitHub
Environment `release`, restrict deployments to `main`, select a maintainer reviewer,
and set its Secret `ATTESTARA_RELEASE_PRIVATE_KEY` to that PKCS#8 PEM. If using an
already authenticated GitHub CLI, stream the file directly into the secret command:

```sh
gh secret set ATTESTARA_RELEASE_PRIVATE_KEY --env release \
  --repo Parsa-Emami/Attestara \
  < "$HOME/.local/share/attestara/release-signing/v1/private.pem"
```

Do not print the file or paste it into chat. Secret configuration is an operator
step, not proof of CI availability; the signing step verifies the key against the
pinned public key. No private key reaches the build/install jobs. Environment
protection must be configured explicitly; declaring its name alone does not
establish a reviewer. Do not run publication on a PR, fork or unreviewed source.

## Reviewable candidate, then explicit publication

1. Review/approve the source change and authorize commit/push separately. Wait for
   all five existing workflows to pass on the exact candidate SHA on `main`.
2. Manually run workflow `release` from `main`, supplying that full `source_commit`
   and leaving `publish=false`. It runs a fresh complete C39 gate, packages the
   exact committed source and retains a candidate for review. A cached green report
   or the temporary test-package commit cannot replace this source.
3. After explicit maintainer publication approval, run the same workflow with the
   same SHA and `publish=true`. Preparation runs again. The protected publish job
   signs verified bytes using the Environment Secret, rechecks main/CI/tag absence,
   creates a draft, uploads/checks all six required files, creates the exact tag,
   and only then publishes the stable release. Only this job has `contents: write`.
4. A separate fresh job downloads the public assets itself, binds tag/source,
   verifies the independent signature and checksums before extraction, and runs
   `npm run test:published -- --source-commit <full-sha>`.

A moving main, missing/running/failed latest workflow, existing tag/release, wrong
key/source, changed bytes or incomplete upload blocks publication. Retries do not
replace a tag or asset. An incomplete draft/tag needs maintainer inspection and an
explicit recovery decision; the workflow never deletes or force-updates it.
Publication and installed acceptance are separate: a failed post-publication test
leaves the release visible and C40 unverified. Retain the failure and stop; do not
claim success or automatically remove public artifacts.

## What the installed test proves

The trusted harness comes from the reviewed source SHA. Runtime bytes come only
from that public release, never a local rebuild or an upstream job artifact. Its
launcher has no Go/npm on PATH; Node remains the documented runtime prerequisite.
The test completes C30 CLI operations/replay/conflict, restart, backup/new-store
restore and the local 360px viewer. The existing C35 helper builds its fixed C34
regression baseline and tests upgrade, newer-data-safe rollback, incompatible
stores and inert input against the downloaded candidate bytes. Its temporary
comparison signatures are distinct from the real maintainer assessment recorded
before extracting the candidate. Go/npm are test-harness tools for this baseline,
not application runtime requirements.

For local validation only:

```sh
npm run test:published -- --source-commit <full-sha> --local-directory <signed-directory>
```

This mode records `mode: local-rehearsal`, `published: false`; it cannot certify
C40. Public mode refuses drafts, prereleases, missing/extra/unsafe assets, a wrong
tag target or corrupt downloads. It stores independent reports under
`dist/published-release/run-*/report.json`, including failed attempts.

Only after actual public installation succeeds may `launch.json`, C40 evidence,
progress, requirements, project state and the generated roadmap be finalized. Bind
all results to the tested commit/tree, tools, signer fingerprint, run/asset URLs,
output hashes and observed recovery. An evidence-only follow-up is not another
runtime run. Keep original source-bound records intact. No npm registry, paid
account, public app hosting or extra platform is part of this release.

## Upstream workflow provenance

Existing checkout/setup/upload pins are reused. The only additional Action is
[`actions/download-artifact` v4.3.0](https://github.com/actions/download-artifact/tree/d3f86a106a0bac45b974a628896c90dbdf5c8093),
pinned to `d3f86a106a0bac45b974a628896c90dbdf5c8093`; it transfers the same run's
candidate to the protected signing job. It is a CI dependency, not bundled runtime
code. No new npm/Go dependency is introduced. GitHub's Node action-runtime
migration warnings do not establish a failed product check.
