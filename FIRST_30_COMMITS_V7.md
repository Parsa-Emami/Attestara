# Attestara — First 30 Commit Plan v7

These are atomic intent boundaries, not mandatory commit-message text. Do not combine unrelated items merely to hit a number.

1. chore: initialize toolchain manifests and lockfiles
2. ci: make repository validation executable
3. chore: add local development environment/health checks
4. test: add clean-bootstrap smoke test
5. docs: freeze Phase 0 invariants and ADR index
6. phase0: define artifact identity by digest
7. phase0: evidence graph and immutable provenance objects
8. phase0: job state machine and idempotency keys
9. phase0: retention/classification rules
10. test(phase0): add digest identity tests
11. docs(phase0): record exit evidence and ADR updates
12. phase1: streaming size limits
13. phase1: path traversal/symlink/special-file defenses
14. phase1: decompression ratio/file-count limits
15. phase1: quarantine before parsing
16. test(phase1): add zip-slip fixtures
17. docs(phase1): record exit evidence and ADR updates
18. phase2: ephemeral rootless Podman runner
19. phase2: no-network default
20. phase2: read-only inputs and bounded writable scratch
21. phase2: cPU/memory/pid/time limits and guaranteed cleanup
22. test(phase2): add network-denial test
23. docs(phase2): record exit evidence and ADR updates
24. phase3: syft adapter pinned by digest
25. phase3: cycloneDX 1.7 normalization
26. phase3: vulnerability source adapter abstraction
27. phase3: record scanner/database versions in evidence
28. test(phase3): add SBOM schema validation
29. docs(phase3): record exit evidence and ADR updates
30. test: close Phase 0/1 regression or failure-proof gap before scope expansion
