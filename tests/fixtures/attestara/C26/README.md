# C26 synthetic bundle trust matrix

Apache-2.0 repository-authored synthetic scenarios. cases.json version 1 defines
exact expected errors and the existing C16 signature booleans. provenance.json
pins these cases and the unchanged C08 manifest/files and C16 public signature.
The C16 key is public fixture material, never a production credential.

Tests build deterministic ZIPs in memory with the exact signed C08 manifest and
real referenced bytes. Modified members receive correct ZIP CRCs: rejection must
come from manifest SHA-256 validation. Whitespace added to manifest bytes preserves
schema and file integrity but invalidates its detached signature. Missing files
fail even when the unchanged manifest signature is valid. Unsigned and valid but
untrusted signatures have distinct raw assessments; only complete file validation
plus a mathematically valid signature and independently configured signer/key
match satisfy the test's bundleTrusted assertion. This is a test assertion, not a
new persisted result, wire schema, CLI capability or public verification API.

The unreferenced source.sh sentinel proves archive content stays inert. Existing
C05/C14 real storage tests retain crash/replay/lost-ACK/quota/ENOSPC coverage; C26
adds no writer or acceptance state. ZIP integrity and manifest signatures do not
establish freshness, revision/replay authority, arbitrary member authenticity,
producer provenance or the C30 browser workflow.
