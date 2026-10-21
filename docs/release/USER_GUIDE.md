# Attestara local user guide

## Install and start

The supported profile is one local Linux x86_64 OS operator using the Go CLI,
immutable snapshots and a browser file viewer. Install Git, Node 24.19.0, npm
11.9.0 and Go 1.27.2. [CLONE_TO_RUN.md](../engineering/CLONE_TO_RUN.md) and
[RESOURCE_LOCK.md](../engineering/RESOURCE_LOCK.md) document pinned resources
and source-build alternatives. No database, Docker, owned domain or paid/cloud
account is needed.

```sh
git clone https://github.com/Parsa-Emami/Attestara.git
cd Attestara
npm run setup
npm start
```

Open `http://127.0.0.1:4173`. Keep that terminal running; Ctrl+C stops the
supervised processes. Use a second terminal in this checkout for the CLI below.
If 4173 is occupied, select `npm start -- --port 4301` and use that URL. Apache
can keep port 80; optional isolated Nginx testing on 8080 is in
[WEB_RUNBOOK.md](../engineering/WEB_RUNBOOK.md#test-alongside-apache-on-port-80).

## Evaluate, verify and view

The included inputs are repository-authored Apache-2.0 synthetic examples,
not scanner findings or evidence about a real product. Use an unused operation
ID and unused output destinations when repeating the guide.

```sh
dist/engine/attestara evaluate --input tests/fixtures/attestara/C30/manifest.json --files tests/fixtures/attestara/C30 --policy tests/fixtures/attestara/C30/rule.json --evidence tests/fixtures/attestara/C30/evidence.json --freshness fresh --store .local-data/attestara --operation-id guide-1 > .local-data/guide-passport.json
dist/engine/attestara verify --store .local-data/attestara --operation-id guide-1
dist/engine/attestara verify --input tests/fixtures/attestara/C30/manifest.json --files tests/fixtures/attestara/C30 --passport .local-data/guide-passport.json --signature tests/fixtures/attestara/C30/signature.json --trust tests/fixtures/attestara/C30/trust.json
```

Open `http://127.0.0.1:4173/#passport` and choose `.local-data/guide-passport.json`.
The browser reads it without uploading. Keyboard navigation reaches the native
file chooser, evidence table, Clear and Cancel controls. The viewer displays
**unverified claims**; it does not authenticate producers or referenced bytes.

Policy, file integrity, cryptographic validity and independently configured
signer trust remain distinct. The example trust keys are synthetic. A changed
or missing referenced file must fail full verification even with a valid
signature. Test mutations in disposable copies, preserving original fixtures.
[LOCAL_WORKFLOW.md](../engineering/LOCAL_WORKFLOW.md) gives exact
formats, signature options, error semantics and independent assessments.

## Preserve and recover snapshots

The default private store is `.local-data/attestara`; an explicit private
`--store` may be kept outside immutable release directories. The same operation
with identical input/result replays; changed content or freshness under that
ID conflicts. A different snapshot needs a new operation ID. No automatic
expiry, latest-release authority or revision ordering is inferred.

```sh
dist/engine/attestara export --store .local-data/attestara --operation-id guide-1 --output .local-data/guide-backup
# Stop/restart npm start, then verify the original operation again.
dist/engine/attestara verify --store .local-data/attestara --operation-id guide-1
dist/engine/attestara restore --backup .local-data/guide-backup --store .local-data/guide-restored --operation-id guide-1
dist/engine/attestara verify --store .local-data/guide-restored --operation-id guide-1
```

Transfer the whole backup directory privately with permissions preserved. Never
copy a live mutable store or overwrite a newer one. Backup/restore rejects every
existing destination, even an empty directory or symlink. After interruption or
uncertain acknowledgement, inspect/reopen before retrying: accepted publication
may already be durable. Preserve original bytes and restore into a new store.
Stop the service before switching immutable release directories. Follow the
[signed-artifact and upgrade procedure](ARTIFACTS.md#verify-a-detached-release-signature-c35)
before using another installed version. Rollback changes the software directory
while retaining the same external store, including snapshots accepted afterward.
Verify the exact expected source and independently established signer before installation;
CI records the compatible baseline/candidate recovery test results.

## Errors and limits

| Observation | Meaning and next action |
| --- | --- |
| CLI exit 0 | The command succeeded/passed its declared checks; inspect separate trust fields. |
| CLI exit 1 | Policy fail/unknown, including missing or stale evidence; do not promote to pass. |
| CLI exit 2 | Invalid input, digest or signature; correct a disposable input copy. |
| CLI exit 3 | Storage/engine/output failure; retain accepted bytes and inspect readiness/destination. |
| Empty/invalid/cancelled viewer | Explicit local state; clear or select a supported passport and retry. |
| Engine unavailable/readiness 503 | Check prerequisites, process and store lock/corruption; web health alone is insufficient. |

ZIP inputs are bounded to 100 MiB and 2,000 entries. The Grype adapter rejects
more than 10,000 matches. The file repository is a bounded single-writer adapter;
see [contracts](../../contracts/README.md) for formats/quotas. The 2 vCPU/2048 MiB resource target does not certify capacity for the complete
combined workload or representative Grype reports. Remote/multi-user domain APIs and
identity/object authorization remain unsupported. OPA and manual Syft/Grype
report ingestion are optional; the core example does not execute those tools
or uploaded user source.

## Privacy, licenses and help

Keep manifests, reports, stores, backups and exported passports private unless
reviewed for disclosure. Redaction recognizes certain credentials/local paths;
it cannot detect every secret. Review exports yourself. Setup downloads public
dependencies; browser-selected passport bytes are not uploaded.
See the [local service, data and privacy policy](SERVICE_POLICY.md) and
[maintenance and dependency policy](MAINTENANCE.md) for lifecycle and support
boundaries.

Project source/examples use [Apache-2.0](../../LICENSE); retain [NOTICE](../../NOTICE).
Dependencies retain their licenses in [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md).
[ARTIFACTS.md](ARTIFACTS.md) covers local artifacts, detached signature verification,
source rebuilds, checksums and bundled notices. The signed v1.0.0 release is available on GitHub.

Ask ordinary questions in [Issues](https://github.com/Parsa-Emami/Attestara/issues)
with the commit, platform/tool versions, exact command and redacted expected/actual
result. Share synthetic reproduction files only. Vulnerability reports and
responses belong in GitHub's private channel described by
[SECURITY.md](../../SECURITY.md); check its activation before sharing details.
No dedicated security mailbox is required.
