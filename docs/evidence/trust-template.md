# Bundle trust evidence template

Planning/recording template only. This file certifies no execution. C26 actual
commands, exits, source identities and results belong in docs/evidence/C26.json.

Record the source revision/tree SHA-256, exact tool binaries and versions,
fixture paths/hashes/license, command stdout/stderr and exits, and test names.
Use the exact manifest bytes from the same ZIP whose members were validated.
Configure signer trust independently of candidate input key material.

| Case | File integrity | Signature | Configured signer/key | Expected assessment |
| --- | --- | --- | --- | --- |
| Trusted | Valid | Valid | Match | All three checks pass |
| Unsigned | Valid | Absent | Not assessed | Unverified |
| Signed but untrusted | Valid | Valid | No match | Unverified |
| Forged/tampered manifest | Valid or rejected | Invalid | May match | Never trusted |
| Modified/missing evidence | Rejected | May be valid | May match | Reject bundle |
| Malformed signature | Valid | Typed error | Not assessed | Reject assessment |

Record raw typed archive errors and C16 booleans, not just a summary label.
A signature authenticates manifest bytes only. Archive inputs retain their
unverified contract field; tests must not rewrite it to claim durable acceptance.
Policy decisions remain independent of authenticity. Unreferenced ZIP members
are inert but are not authenticated by the manifest's referenced-file digests.

Map required recovery negatives to actual C05/C14 storage tests: staged-byte/
manifest-boundary SIGKILL, reopen, idempotent replay/lost ACK, injected ENOSPC and
quota rejection. Rerun these regressions with C26; never relabel them as new
trust-authority or physical power-loss proof. Record unavailable providers and
skipped tests explicitly. C26 itself needs no external provider.

Do not infer CLI/viewer integration, freshness/expiry/replay authority, clean-host
workflow, remote CI, publication or C28 hardware capacity from this matrix.
