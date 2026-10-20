# C39 release-gate fixtures

Apache-2.0, repository-authored synthetic metadata. These cases exercise evidence
validation; synthetic C01-C38 records in unit tests certify no product behavior.
The complete gate separately runs the actual storage, engine, browser, clone,
packaging and installed upgrade/recovery checks. Recorded command text stays inert.

Missing/changed evidence, string success, templates, duplicate or missing tasks,
unsafe/symlink paths, wrong source, corrupt logs, shallow history, skipped or
unstarted Go tests, nonzero process exits, cancellation and timeout must fail.
Existing real C05/C14 tests cover SIGKILL, lost ACK, replay and ENOSPC/quota;
C30/C31/C35 exercise inert source, package publication and new-store recovery.

Historical C32 framing, C36 ZIP/member digests and C37 documentation overlays are
explicit compatibility records. C31's retained typecheck log omitted its last LF;
the source-bound C39 revalidation reproduces the original expected output digest.
