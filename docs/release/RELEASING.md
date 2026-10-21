# Maintainer releases

Supported target: Linux x86_64, one OS operator, `local-cli-snapshots`.
`RELEASE_MANIFEST.json` pins the version/profile and independently established
Ed25519 release public key. Existing tags/assets are never overwritten.
The published v1.0.0 remains bound to its original source commit; a later source
cleanup does not replace that release or its signed archives.

## Key and Environment

Establish the public-key fingerprint independently before configuring trust.
The current fingerprint is
`sha256:d02ef875aeff974b033195e4b1b1b80491d16145e79bf7e603a4985c927606cd`.
The private PKCS#8 key belongs outside Git/artifacts/logs at
`~/.local/share/attestara/release-signing/v1/private.pem`, with directory mode
0700 and file mode 0600. Keep a private backup. Never print it or silently
replace a missing key. Synthetic test keys are not the release identity.

The GitHub Environment `release` must restrict deployments to `main`, require a
maintainer reviewer and contain Secret `ATTESTARA_RELEASE_PRIVATE_KEY`.
An authenticated repository owner can check the current configuration, or apply
it explicitly from the machine that already holds the matching private key:

```sh
node scripts/configure-release-environment.mjs --reviewer Parsa-Emami
node scripts/configure-release-environment.mjs --apply --reviewer Parsa-Emami
```

The helper validates the existing key/public identity before writes, preserves
existing reviewers and review timing, and sends the PEM only through secret-command
stdin. It never generates a replacement, dispatches a workflow or approves a
release. Re-read configuration/Secret metadata and verify actual signing. Review
administrator bypass separately in Environment settings; keep bypass disabled.

## Prepare, approve and publish

1. Review and commit the intended source on main. Require successful current-SHA
   contracts, engine-runtime, web-foundation, release-gate and github-pages workflows.
2. Dispatch `release.yml` with that full source SHA and `publish=false`. Preparation
   runs the current-source gate and packages the exact clean commit. Review its
   retained candidate artifact and fresh report.
3. With explicit maintainer publication authorization, dispatch the same workflow
   with the same SHA and `publish=true`, then approve the protected Environment.
4. The independent installed job downloads the actual public assets, verifies
   expected source/signature/checksums before extraction and tests the local workflow,
   restart, restore and compatible software rollback retaining newer store data.

```sh
candidate_sha=$(git rev-parse HEAD)
gh workflow run release.yml --repo Parsa-Emami/Attestara --ref main \
  -f source_commit="$candidate_sha" -f publish=false
```

Only after reviewing that candidate and authorizing publication, reuse the same
SHA with `-f publish=true`. The latest matching preparation must have successful
`prepare`, skipped `publish`/`installed`, and an unexpired candidate artifact.
Publication requires the complete ordered current-source gate report and successful
product workflows; a cached result or different SHA cannot satisfy the gate.

Preparation/install jobs receive no private key. Only the protected publish job
has repository write permission. It signs exact verified SHA256SUMS bytes, creates
a draft, verifies all six uploaded assets, creates the exact tag and publishes.
A moving main, existing tag/release, wrong key/source, incomplete upload or failed
check stops publication. Retry never deletes or force-updates an existing object;
inspect incomplete attempts and decide recovery explicitly.

A published release and its installed acceptance are separate. A failed installed
test leaves the release visible and must be reported as a failure. CI retains
fresh reports and command outputs as run artifacts instead of adding development
progress or execution logs to the repository.

## Installed validation

The trusted harness is read from the reviewed source. Candidate runtime bytes
come from the public release, not a rebuild. The launcher uses Node without Go/npm
on its runtime PATH. Go/npm are harness prerequisites for the fixed compatible
baseline used by the upgrade/rollback regression.

```sh
npm run test:published -- --source-commit EXPECTED_FULL_SOURCE_COMMIT
```

For a local signed candidate only, add `--local-directory /absolute/candidate`.
That mode records `published: false` and cannot certify public installation.
Public mode rejects drafts/prereleases, unsafe or incomplete assets, wrong tag/source
and altered downloads. Reports are under `dist/published-release/run-*/report.json`.
The workflow transfers candidates using SHA-pinned Actions without introducing a
new npm/Go runtime dependency. See [ARTIFACTS.md](ARTIFACTS.md) for verifier/trust
setup and safe installation.
