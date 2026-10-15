# Attestara — Final Implementation Specification

This specification freezes the v1 implementation contract. Changes that violate an invariant, authority boundary or public contract require an ADR/RFC and migration/compatibility plan.

## Domain objects
- `Repository`
- `SourceSnapshot`
- `Subject`
- `ScanRun`
- `RunnerLease`
- `ScannerAdapter`
- `EvidenceEnvelope`
- `SBOM`
- `VulnerabilityFinding`
- `Policy`
- `PolicyDecision`
- `Attestation`
- `ReleaseCandidate`
- `Exception`

## Aggregate rules
- Aggregate mutations occur through application commands, never controller/model convenience writes.
- Aggregate version is checked on conflicting writes.
- Invariants are tested with illegal-transition/negative cases.
- Domain objects do not import Laravel/Eloquent/vendor SDKs.

## Frozen v1 invariants
1. Untrusted source execution is isolated from control plane.
2. Raw and normalized evidence are digest-linked and immutable.
3. Policy consumes normalized schema, never scanner-specific untrusted structure directly.
4. Runner is ephemeral and receives no production credentials.
