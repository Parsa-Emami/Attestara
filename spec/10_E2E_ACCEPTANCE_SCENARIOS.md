# Attestara — Final Implementation Specification

This specification freezes the v1 implementation contract. Changes that violate an invariant, authority boundary or public contract require an ADR/RFC and migration/compatibility plan.

## Golden E2E scenarios
1. Safe archive -> isolated Syft -> CycloneDX evidence -> OPA decision
2. Kill runner and verify no orphan + retry/quarantine
3. Reject archive traversal/bomb fixture
4. Tamper normalized evidence and fail verification
5. Release passport verifies SBOM/provenance/signature references

For every golden scenario record expected authoritative state, emitted event IDs/types, audit action, projection state, critical metrics and cleanup state. Run from a clean environment with deterministic synthetic fixtures. At least one scenario must fail mid-flight and then converge after restart/retry/reconciliation.
