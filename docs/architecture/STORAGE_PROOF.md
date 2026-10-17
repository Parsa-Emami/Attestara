# C05: recoverable local authoritative writes

C05 implements the ADR-0003 file repository in `internal/storage`. Its proof is
`docs/evidence/C05.json`. It consumes actual C04 release inputs/results and artifact
bytes; it does not evaluate a policy or verify producer identity/signatures.
The owner approved an internal storage-library scope. C05's copied identity,
header and browser-admission cases were replaced with N05 storage cases in the
roadmap source and task card. The existing HTTP/UI admission remains unchanged.

## Port and exact bytes

```go
store, err := storage.Open(privateDirectory) // existing parent; caller owns lifecycle
// Handle err before using store; defer store.Close() after successful Open.
err = store.Commit(ctx, storage.Record{
    OperationID: stableOperationID,
    InputJSON: exactInputBytes,
    ResultJSON: exactResultBytes,
    Files: manifestPathToBytes,
})
record, err := store.Load(ctx, stableOperationID)
```

`Port` consists of Commit, Load and Close. Record retains the original JSON bytes
because C04 defines input_digest over exact input bytes, including whitespace.
The adapter calls existing DecodeInput/DecodeResult to obtain validated typed
values. It checks input_digest and result subject/evidence/policy binding, plus
every file's actual size/SHA-256. Missing/extra files fail. Owner remains metadata.
The caller must not mutate slices/maps during Commit. Load returns independent
validated bytes. Manifest paths never become host filesystem paths.

An operation ID is a nonblank valid UTF-8 string of at most the existing 4096-code-
point text bound. It never names a host file. The same ID with identical input and
result bytes returns success without changing the manifest. Different bytes under
that ID return ErrConflict. Artifact content is already bound by the input hashes.
Failed domain results may be stored; storage is not evidence that their producer ran.

## Visible transaction and file layout

The private root (0700), its objects/staging directories and managed regular files
must belong to the current effective UID; managed files have no group/other access
and no additional hard links. New files are 0600. The adapter uses Go os.Root for
root-relative confinement and O_NOFOLLOW for regular file opens. It refuses a
symlink root, shared permissions, unsafe entries and unknown managed filenames.
This is not a boundary against a hostile process with the same OS credentials.

- `lock`: an exclusive nonblocking Linux flock held from Open through Close.
  A second process fails with ErrLocked; the OS releases the lock after SIGKILL.
  A per-instance TryLock rejects concurrent calls without an internal work queue.
- `objects/<64 lowercase SHA-256 hex>`: immutable artifact bytes, deduplicated by
  digest. Existing bytes are hash/size checked rather than overwritten.
- `staging/blob`: the single bounded in-progress artifact file.
- `manifest`: newline-delimited versioned snapshot. Its exact first line is
  `{"schema_version":1}`; each following line holds operation_id, base64-encoded
  exact input/result bytes and a SHA-256 checksum of that envelope without its
  checksum field. This checksum detects corruption; it is not authentication.
- `manifest.next`: the uncommitted replacement snapshot.

Commit validates contracts and all supplied bytes, stages/syncs each artifact,
renames artifacts into objects, and syncs both objects and staging directories.
It writes the old manifest plus the new operation into manifest.next, syncs and
closes it, atomically renames it over manifest, then syncs the root directory.
Only then does Commit return success. The result/status and operation replay
state appear in the same publication; there is no separate queue or job database.
Open also syncs recovery changes and the parent directory before returning.

File sync alone does not persist a directory entry; see the Linux
[fsync documentation](https://man7.org/linux/man-pages/man2/fsync.2.html).
Snapshot replacement follows local-filesystem
[rename semantics](https://man7.org/linux/man-pages/man2/rename.2.html), and
writer exclusion follows [flock semantics](https://man7.org/linux/man-pages/man2/flock.2.html).
Go Root behavior was inspected in the C02-pinned Go source `src/os/root.go`.

## Failure, replay and recovery

Any failure after writes begin invalidates the handle: subsequent Commit/Load
returns ErrRecovery until Close/Open. No rollback is attempted after publication.
A directory-sync error can leave a visible new manifest without acknowledgement.
Reopen flushes/reconciles that state; retrying the same operation ID resolves it
without duplicating an accepted operation. Cancellation before publication aborts;
once published, directory flush completes before returning success even if the
caller cancels. The caller should retain the operation ID when no acknowledgement
arrives.

Open first validates every committed record, duplicate ID and referenced object.
Missing/corrupt references, unsupported manifest versions, unknown/duplicate JSON
fields, invalid checksums and truncated lines fail closed. It never discards an
accepted record, substitutes an empty manifest or selects an older backup.
Only after that check does it remove owned staging/manifest.next files and
unreferenced digest-named objects. An orphan's hash must match its name before
removal. Unknown entries are preserved and rejected. Partial initial header bytes
can be reconciled only when no manifest or published object exists.

Stable Go sentinels distinguish ErrLocked, ErrConflict, ErrNotFound, ErrCorrupt,
ErrInvalid, ErrUnavailable, ErrRecovery and ErrClosed. C04 validation errors retain
their existing codes. I/O errors preserve errors.Is for errno such as ENOSPC without
including user payload bytes. These are internal library errors, not HTTP codes.

## Bounds, reproduction and limits

C04's 100 MiB per JSON document, 100 MiB total artifact bytes and 2000 files remain
in force. The manifest is scanned/rebuilt incrementally rather than retained as an
unbounded in-memory map. A line is capped at 3 times the C04 wire byte limit to
accommodate base64 and the envelope. A constant number of bounded records may be
resident during nested validation. Directory enumeration is batched. Accepted
history and stable operation IDs are never evicted; a full volume rejects writes.
There is no new operation-count quota, retention policy or background cleanup.

Recovery uses repeated scans to retain bounded memory (quadratic history/objects
work is possible); each commit rewrites the manifest. C28 must measure capacity
and latency before an index or another adapter is justified. No minimum-memory or
throughput claim is made here. A complete-record checksum is not a signed journal
and cannot detect deletion of a whole trailing record by a privileged editor.

Use the existing pinned Node/npm/Go toolchain and run:

```sh
go test ./...
go vet ./...
gofmt -l internal/storage/adapter.go internal/storage/adapter_test.go tests/storage_test.go
npm run check
```

The C05 fixture directory contains Apache-2.0 synthetic scenario/expected-output
metadata and reuses exact C04 artifacts. Real temporary filesystems, independent
Go subprocesses and SIGKILL exercise writes, restart, locking and recovery. An
unexported test checkpoint injects ENOSPC before individual write/sync/publication
calls; the rest of the persistence path uses real files. No physical volume is
filled, no power-cut/block-device experiment is claimed, and no external provider
is needed. Filesystem flush guarantees still depend on the kernel, filesystem and
storage device honoring sync. The measured host/filesystem is recorded in evidence.
This is the supported Linux local adapter; NFS, multiple hosts, additional platforms,
private API identity and full C30 workflow/backup restore remain separate work.

Rollback before accepting data may revert this isolated code. After accepting
state, preserve its latest valid bytes and compatible reader; repair forward.
Never replace newer accepted state with an older export. No schema, migration,
.env, Dockerfile, dependency or existing RiskWeft API changes are required.
