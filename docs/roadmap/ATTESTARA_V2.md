# Attestara 2.0 — Open-source engineering and release roadmap

**Baseline:** 2026-11-23. **Status:** v1.0.0 exists; v2 work has begun, no v2 tag/release or SLSA-certification claim. Goal: freely redistributable Apache-2.0 source, portable offline local verification, no required SaaS or commercial API.

| Phase | Priority | Implementation | Blocking acceptance evidence |
| --- | --- | --- | --- |
| V2-00 | P0 | v1 contracts, source audit, version/OSS dependency inventory and CI baseline | Exact SHA, reproducible toolchain, failures recorded |
| **V2-01** | **P0** | **In-toto Statement v1 / SLSA provenance v1 subject-to-manifest binding, checked local bytes, bounded parser, CLI and negative tests** | Implemented in this commit; independent CI verification still required |
| V2-02 | P0 | DSSE envelope verification, independently configured identity/key trust | Forged/unsigned/wrong-key/mismatched signatures rejected |
| V2-03 | P0 | SLSA predicate validation with explicit builder/source policies | Build definition/runner/identity fields checked, fail closed |
| V2-04 | P0 | CycloneDX subject and provenance binding | SBOM tampering, stale and replacement tests |
| V2-05 | P0 | Versioned policy engine for provenance decisions | Pass/fail/unknown deterministic reason codes |
| V2-06 | P0 | Explicit separation of claim, byte integrity, signer trust and builder trust | No trust promotion in CLI, reports or viewer |
| V2-07 | P1 | Portable CI adapter and optional GitHub Action | Same checks runnable without GitHub |
| V2-08 | P1 | Deterministic JSON/SARIF reports and safe redaction | Stable schemas and no leak of private metadata |
| V2-09 | P0 | Attack-oriented offline file/ZIP, signature and metadata tests | Traversal, symlink, duplicate, zip bomb and replay denials |
| V2-10 | P1 | Non-root packaging, checksums, SBOM and source reproducibility | Independently reproducible signed release candidates |
| V2-11 | P1 | Trust-aware browser inspection UX and accessibility | Viewer never claims verified signer/bytes; browser E2E |
| V2-12 | P0 | Full security, dependency, clean-clone and release acceptance | Go, Node, browser, package and rollback gates green |
| V2-13 | P0 | Public v2 release, migration docs and maintenance | Maintainer-approved tagged assets and anonymous verification |

**Critical path:** V2-00 → V2-01 → V2-02 → V2-03 → V2-04 → V2-05 → V2-06 → V2-09 → V2-12 → V2-13. V2-07/08/10/11 must also finish before the public v2 gate.

## Non-negotiable OSS and security constraints

- Retain first-party Apache-2.0 licensing, third-party notices, pinned public inputs and build instructions; no mandatory paid account, closed-source scanner, proprietary runtime or GitHub-specific execution path.
- Local-first, synthetic fixture data; CLI does not upload release metadata or execute a predicate.
- Policy decisions, file bytes, signatures and independently configured builder identity must remain separate results. SLSA predicate metadata matching is **not** a SLSA build level or attestation-authenticity claim.
- Each phase requires tests, negative scenarios, reproducible command, actual commit SHA and CI URL. Do not change previously released v1.0.0 binaries or signed assets.
- Do not issue a v2 release until the existing release gate plus new v2 regressions, legal checks and independent source-build validation pass.

**Next work item:** V2-02 (DSSE signature verification and independent trust anchors), followed by V2-03 builder policy.
