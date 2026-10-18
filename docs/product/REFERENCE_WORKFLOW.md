# Attestara: reference workflow

Execution status: unimplemented. This document and `tests/scenarios/clone-to-value.json` define C01 acceptance only; no complete engine, provider, verifier/viewer parity or durable restore has passed this workflow. The web application has a bounded JSON structure preview and a browser-local C19 passport viewer.

## Synthetic release and ownership

The repository owns the Apache-2.0 synthetic fixture descriptions under `tests/fixtures/attestara/C01/`. `positive.json` identifies one release, its synthetic maintainer, source revision, subject bytes, a CycloneDX JSON SBOM description, and the subject/evidence/policy SHA-256 digests. `negative.json` describes deliberate mutations. `expected.json` records the portable passport fields and one observable outcome per scenario action. The SBOM and policy byte strings are labels for this specification; C04 now supplies separate concrete synthetic artifacts and real Go contract validators under `tests/fixtures/attestara/C04/`; it does not execute this workflow, evaluate policy or establish authenticity. No customer data, secret, user source execution or external account is involved.

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


C14 adds an internal, persisted first-party `evidence_present` predicate, with
pass/fail/unknown and exact rule/evidence references under result v2. Its synthetic
fact documents are concrete evaluator inputs, not scanner/trust assertions. This
is one engine slice; the ten-step browser/passport/signature/restore workflow
above remains unexecuted. See `contracts/README.md` for the approved C14 boundary.

C15 adds an internal optional OPA 1.21.2 CLI adapter for the exact C04 allow
policy and C14 fact format. Real OPA decisions produce the existing v2 result;
timeout is an explicit failed evaluation and authenticity stays unverified.
No scanner-specific schema, arbitrary policy execution, HTTP admission or browser
workflow is added. The C04 policy defaults missing/null facts to false; C14's
first-party unknown behavior remains unchanged.

C16 adds an internal stateless Ed25519 verifier for exact C08 v1 manifest bytes.
It returns independent mathematical validity and configured signer/key membership;
Trusted() requires both. A key supplied with input never grants trust by itself.
The assessment authenticates only manifest bytes: callers must compose C08
real-file validation before reporting bundle verification. C04/C15 wire results
remain unchanged. No expiry/revision ordering, accepted-authority replacement or
replay state is implemented. C20's CLI verifies only a passport digest; it does
not add stateful acceptance. CLI/viewer integration remains C20/C21.

C17 adds internal stateless comparison of the existing C07 envelope subject digest
with the existing C08 manifest subject digest. SubjectBound reports only equality
of validated claims: valid mismatch is false without error; missing/invalid input
fails with a safe typed error. C08 actual-file validation and C16 signer trust
remain independent. Builder/source assertions stay unverified without issuer
policy. No new provenance format, authority state, CLI or viewer is added;
freshness/replay authority remains a required but unimplemented workflow
obligation. Future C18/C19 decisions/passports can compose these independent assessments.

C18 adds stateless internal decision.Explain over existing v2 evaluation results.
Caller-classified stale evidence always yields unknown, even with a zero-findings
C12 report and C14 pass. Existing warnings carry evidence_missing, evidence_stale
and policy_failed reasons. Missing references stay unknown; raw parser/provider
failures retain their typed errors. No age threshold or authenticity is inferred.
The caller owns freshness classification; C19-C21 provide subsequent passport,
CLI and viewer composition. This does not establish durable freshness/revision
authority or replay protection.

C20 adds local `evaluate`, `verify` and `export` commands. `evaluate` composes
the existing C14 predicate, explicit caller-classified C18 freshness and C19
passport generation. Missing or stale evidence remains unknown, and every
passport remains unverified. `verify` and `export` validate the C19 version and
canonical passport digest; neither verifies referenced file bytes or signer
trust. All commands preserve non-pass passports on stdout and return the policy
non-pass exit code. The commands do not store accepted revisions or prevent replay. The
full workflow's subject-byte verification, signer trust and durable
revision/expiry/replay behavior remain unimplemented and must be specified and
verified before C30 can certify clone-to-value.

C21 adds a browser-local viewer for version-1 C19 passports. It renders policy,
verification and evidence-reference claims as inert text; a selected file stays
in tab memory, with explicit empty, invalid, cancelled and engine-unavailable
states. The viewer does not check the passport digest or referenced files and
does not establish signer trust. C22-C26 own the separate diff, export,
scanner, privacy and trust obligations.

C23 adds a local ZIP export mode for the existing v1 manifest and its referenced
files. The exporter validates source bytes, builds the C09 archive shape and
checks the complete ZIP with `bundle.ReadArchive` before publishing a new file.
The offline reader rejects absent or modified members after export. This proves
portable file integrity for the exported inputs only; passport binding, signer
trust and durable revision/replay authority remain separate obligations.
