# C16 synthetic signature fixtures

Apache-2.0 repository-authored data. positive.json contains an Ed25519 public
key and detached signature over the exact unchanged C08/positive.json bytes.
The fixture-only seed was bytes 0 through 31; it is public synthetic test data,
never a production key. Fixture signatures were generated independently with
Python cryptography; Go verifies them using its standard library.

expected.json separates cryptographic validity from configured signer/key
identity membership. Trusted() requires both. A trusted policy match alone
does not authenticate a modified payload. negative.json describes typed failures.

C16 checks manifest syntax and signatures, not physical evidence availability.
Callers must compose the existing C08 ReadRoot/ReadArchive validation before
using an assessment as bundle verification. No manifest fields, acceptance
state, expiry, revision ordering or replay authority are introduced. Replaying a
valid signed manifest can remain cryptographically valid; it cannot update state
through this stateless API. Stateful acceptance must add freshness/replay rules.
