# C19 deterministic passport fixtures

These Apache-2.0 repository-authored synthetic cases reuse the verified C14
manifest, evidence and rule bytes. The positive test adds a second manifest path
for the same inert evidence bytes, then runs the real C14 evaluator and C18
decision explanation with both file orders. C19 sorts the resulting references
before computing the version-1 passport digest.

The passport digest is SHA-256 over compact JSON for the typed passport payload
in struct field order, excluding `report_digest`; evidence references are sorted
by path and then digest. Policy warnings retain C18 order. The passport preserves
the independent `unverified` status and makes no file-integrity, signer-trust,
revision-authority, persistence or source-execution claim.
