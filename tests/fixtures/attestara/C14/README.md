# C14 synthetic release-rule fixtures

Repository-authored Apache-2.0 data. `rule.json` is the single version-1
`evidence_present` predicate. `positive.json` maps concrete manifest/fact files
to pass/fail/unknown expectations. Each fact has independently hashed exact bytes;
false cannot be substituted for true without changing the input evidence digest.
Unknown facts omit the boolean. Unavailable bytes also yield unknown in memory,
while the durable C05 store still refuses incomplete bundles.

`negative.json` holds malformed/unknown-version/source-like rule cases. Tests also
exercise bound malformed fact bytes, references, cancellation and byte changes.
The v2 schema is tested against the existing offline JSON Schema validator.
C14 storage tests use the existing real C05 filesystem port and checkpoint
harness: SIGKILL (real subprocess), ENOSPC (injected), lost-ACK/replay and an
on-disk manifest at the quota limit. These do not simulate physical power loss
or claim actual volume exhaustion. Temporary test stores contain synthetic data.

No HTTP/UI admission, scanner/provider/OPA execution or authenticity claim.
`provenance.json` inventories every fixture other than itself by exact SHA-256.
