# Attestara — Final Engine Decisions v7

These are accepted v1 decisions, not open blockers. Alternatives require a future ADR and must preserve public/domain contracts.

1. Repository code is never executed in Laravel/control-plane workers.
2. Non-executing scanners run in ephemeral rootless Podman containers: read-only rootfs, dropped caps, no-new-privileges, seccomp, quotas, network disabled unless scanner adapter declares a reviewed requirement.
3. Any feature that executes repository build/test code uses a Firecracker-class microVM runner; it is a distinct trust tier and cannot reuse scan-runner credentials.
4. CycloneDX 1.7 is primary normalized BOM output. SPDX 3.0 is a supported import/export format.
5. EvidenceEnvelope binds subject digest, source commit, scanner name/version/image digest, config hash, timestamps, raw-output digest and normalized-output digest.
6. SLSA 1.2 Build L2 evidence is the stable-v1 target; no higher level is claimed without builder evidence.
7. OPA policies evaluate normalized evidence; scanner-specific fields never leak into domain policy directly.
8. Release artifacts carry SBOM, VEX where applicable, in-toto/SLSA attestations and Sigstore/Cosign verification instructions.
