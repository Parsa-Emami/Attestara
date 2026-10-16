# Attestara — Definitive Sequential Roadmap v7

**Portfolio order:** 7 / 10
**Complexity:** Very high
**Planning estimate:** 14–20 focused weeks (not a promise; recalibrate after Phase 1)

**Why this order:** High-value project but unsafe if attempted before sandboxing, artifact handling and provenance primitives are proven.

## First executable proof
Source upload → safe extraction → no-network Syft scan → CycloneDX evidence → OPA decision → runner destruction/cleanup proof.

## Rules for this repository
- One phase at a time; no implementation work for phase N+1 before phase N exit evidence is merged.
- Contract/schema changes precede dependent implementation.
- Every retryable operation is idempotent.
- Every derived store/projection has a rebuild path.
- Negative and failure tests are part of the feature, not post-release hardening.
- Status claims must be evidence-backed.

## Phase 0 — Artifact/evidence model

**Implementation deliverables**
- Define artifact identity by digest
- Evidence graph and immutable provenance objects
- Job state machine and idempotency keys
- Retention/classification rules

**Required verification**
- digest identity tests
- state machine tests
- duplicate job tests

**Definition of Done**
- Artifact/evidence identities are content-addressed and job retries do not duplicate evidence.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 1 — Safe archive intake

**Implementation deliverables**
- Streaming size limits
- Path traversal/symlink/special-file defenses
- Decompression ratio/file-count limits
- Quarantine before parsing

**Required verification**
- zip-slip fixtures
- zip-bomb limits
- symlink escape tests
- malformed archive fuzz

**Definition of Done**
- Host filesystem cannot be escaped and oversized/malicious archives fail before analysis.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 2 — Rootless scan runner

**Implementation deliverables**
- Ephemeral rootless Podman runner
- No-network default
- Read-only inputs and bounded writable scratch
- CPU/memory/pid/time limits and guaranteed cleanup

**Required verification**
- network-denial test
- resource-limit tests
- cleanup leak test

**Definition of Done**
- Scanner executes without network/privilege and leaves no container, mount, secret, or writable artifact after completion.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 3 — SBOM + vuln adapters

**Implementation deliverables**
- Syft adapter pinned by digest
- CycloneDX 1.7 normalization
- Vulnerability source adapter abstraction
- Record scanner/database versions in evidence

**Required verification**
- SBOM schema validation
- normalization fixtures
- offline replay tests

**Definition of Done**
- Same artifact and scanner/database versions reproduce the same normalized evidence payload.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 4 — OPA policy engine

**Implementation deliverables**
- Versioned policy bundles
- Strict input schema
- Decision explanation and rule provenance
- Fail-closed behavior for invalid/missing bundle

**Required verification**
- policy unit tests
- invalid bundle tests
- decision determinism

**Definition of Done**
- Every policy decision records exact bundle digest and cannot silently evaluate against stale/invalid policy.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 5 — Attestation/provenance/signing

**Implementation deliverables**
- Generate in-toto/SLSA-compatible provenance payload
- Keyless or OpenBao-backed signer abstraction
- Verify signatures before trust
- Separate build identity from user identity

**Required verification**
- signature tamper tests
- provenance verification tests
- key rotation tests

**Definition of Done**
- Attestations verify cryptographically and reference immutable artifact/evidence digests.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 6 — Firecracker execution tier

**Implementation deliverables**
- Isolated build execution adapter
- Ephemeral disks/network policy
- Snapshot/image provenance
- Host capability checks

**Required verification**
- runner contract tests
- isolation config tests
- crash cleanup tests

**Definition of Done**
- Build execution meets a stricter documented isolation profile than scan-only runners and cleans up after crash.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 7 — VEX/exceptions/passport

**Implementation deliverables**
- VEX lifecycle and justification evidence
- Time-bounded exceptions with approval
- Artifact passport bundle
- Verifier CLI/input contract

**Required verification**
- expiry tests
- approval auth tests
- passport tamper tests

**Definition of Done**
- Exceptions expire automatically and passport verification detects any modified evidence component.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Phase 8 — SLSA/public v1

**Implementation deliverables**
- Map implementation to SLSA 1.2 claims without overclaiming level
- Reproducible demo pipeline
- SBOM for Attestara itself
- Signed release and runner security review

**Required verification**
- clean-host e2e
- release verification
- threat-model closure

**Definition of Done**
- Public v1 states only evidence-backed supply-chain guarantees and reproduces end-to-end on a clean supported host.
- CI is green from a clean checkout.
- New behavior is observable without logging secrets/sensitive payloads.
- Failure behavior and rollback/recovery note are documented.
- Contracts and README status are updated if externally visible behavior changed.

## Public v1 invariant
The project is not called production-ready merely because all phases are coded. `PUBLIC-V1-GO` requires the repository release gate, clean-install proof, security/recovery evidence and signed/verifiable release artifacts.
