# Local service, data and privacy policy

Attestara's verified profile is one local Linux x86_64 OS operator using the
`local-cli-snapshots` workflow and a browser file viewer. It is not a hosted or
multi-user service. No paid account, public domain, database or cloud service is
required for the core workflow.

## Where data lives

The default private snapshot store is `.local-data/attestara` in the checkout.
An operator can pass another private directory with `--store`; the store should
be outside immutable release directories. Input manifests, reports, source
files, exported passports and backups remain at the paths selected by the
operator. The browser passport viewer reads the selected local file without
uploading it. The local listener binds to loopback; this does not provide
identity or object-level authorization for remote users.

Treat manifests, reports, snapshots, exports and backups as private. Passport
redaction recognizes some credentials and local paths, but cannot find every
secret; review output before sharing it. The CLI and viewer do not execute
selected user source. See the [user guide](USER_GUIDE.md),
[storage proof](../architecture/STORAGE_PROOF.md) and [network boundary](../../infra/deployment.md)
for implementation details and their limits.

## Export, retention and deletion

The CLI exports one operation at a time. Export the operations you need; it is
not a whole-store backup. Accepted snapshots have no automatic expiry or
retention schedule. Attestara has no in-product delete command. To remove data,
stop the service and remove the relevant local files or directories yourself,
including any separate copies and backups. No secure-erasure guarantee is made
for filesystems, snapshots or storage media.

Deleting a snapshot object directly from a live store can corrupt its manifest
references. Preserve the complete store while it is active; export first if the
operation must be retained. When uncertain about accepted state or an
acknowledgement, reopen and verify before retrying. These boundaries follow the
[CLI workflow](../engineering/C30_LOCAL_WORKFLOW.md) and [recoverable storage contract](../architecture/STORAGE_PROOF.md).

## Backup and recovery

Use the CLI export command to create a private backup for each operation. Stop
the service before copying or moving store data; do not copy a live mutable
store. Keep permissions and the full exported directory, and protect backups
like the source data. Restore creates a new store and refuses an existing
destination. Verify the restored operation before relying on it. Never restore
over a newer store or treat a software rollback as a data rollback: the
supported upgrade/rollback procedure preserves the same external store and any
newer accepted snapshots. See [artifact upgrade and rollback](ARTIFACTS.md#upgrade-and-roll-back-without-replacing-accepted-data).

The recovery guarantee is limited to the tested local Linux filesystem and
single-writer profile. It is not a distributed backup, remote recovery service,
or guarantee against device loss. Keep an independent protected copy when
availability matters.

## Supported profile and support

The verified profile is Linux x86_64, one local OS operator, immutable CLI
snapshots and a browser-local passport viewer. Other operating systems,
multi-user access, hosted sensitive workloads, remote domain APIs, identity and
object authorization are unsupported until separately implemented and tested.
The optional OPA and Syft/Grype integrations are not required for the core
workflow. Capacity evidence applies only to the workloads explicitly recorded
in project evidence; it is not a general workload guarantee.

Use public GitHub Issues for ordinary support and never include secrets,
private source, live stores or incident data. Vulnerability reports belong in
the enabled private [GitHub advisory channel](../../SECURITY.md); use that
channel only while it remains available. No support SLA or stable published v1
is claimed. The exact current support boundary is also recorded in
[`SECURITY.md`](../../SECURITY.md#supported-scope).
