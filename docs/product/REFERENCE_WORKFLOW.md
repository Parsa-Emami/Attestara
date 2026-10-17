# Attestara: reference workflow

Execution status: unimplemented. This document and `tests/scenarios/clone-to-value.json` define C01 acceptance only; no engine, provider, verifier, viewer or durable restore has passed this workflow. The current web application is a bounded JSON structure preview.

## Synthetic release and ownership

The repository owns the Apache-2.0 synthetic fixture descriptions under `tests/fixtures/attestara/C01/`. `positive.json` identifies one release, its synthetic maintainer, source revision, subject bytes, a CycloneDX JSON SBOM description, and the subject/evidence/policy SHA-256 digests. `negative.json` describes deliberate mutations. `expected.json` records the portable passport fields and one observable outcome per scenario action. The SBOM and policy byte strings are labels for this specification; C04 must provide concrete domain artifacts before an engine run can be claimed. No customer data, secret, user source execution or external account is involved.

## Bounded workflow

| Action | Expected state and output | Failure behavior |
| --- | --- | --- |
| Import the subject manifest and SBOM evidence | One owned release retains the source revision and subject, evidence and policy digests. | Reject missing or inconsistent manifest fields. |
| Evaluate the local policy | Bind its decision to the subject and evidence; complete synthetic evidence is expected to allow. | Missing evidence is unverified, never pass. |
| Produce a portable passport | Record the subject digest, policy result and separate authenticity status. | Reject mismatched subject or evidence binding. |
| Change one subject byte and verify | Earlier passport remains bound to the original subject; modified bytes fail verification. | Fail closed if the changed bytes cannot be read. |
| Import an incomplete manifest | No release is accepted. | Reject absent evidence digest. |
| Inspect unsigned, invalid-signature and signed-but-untrusted cases | Unsigned and untrusted remain unverified; invalid signature is rejected or explicitly unverified. | A signature's presence or a hash alone never establishes producer trust. |
| Compare offline verifier and viewer | Both show the same policy result for the same passport. | Report disagreement explicitly. |
| Restart and restore | Accepted passport retains the subject digest and policy result. | Reject missing or corrupted restored evidence. |

The exact ten actions, fixture references, expected states/results and errors are in the machine contract. A trusted result requires cryptographic signature verification and a trusted signer; C01 supplies neither. Policy allow and authenticity verification are independent.

The reference invariants are: no user source execution; digest binds subject and outputs; missing evidence never means pass. This resolves the earlier `negative_uat` list, which mixed positive and negative expectations. The requirements in `product/project.json` remain the source for subject binding, portable passport, changed-subject rejection and viewer/verifier parity.

`node scripts/verify-reference.mjs` checks only this specification and its synthetic fixture metadata. C30 must later run the full workflow on a clean supported Linux clone using `npm run setup` and `npm start`, including actual restart and restore. C01 does not make either command proof of domain behavior. C04 may extend the initial fixture format while preserving the C01 contract.
