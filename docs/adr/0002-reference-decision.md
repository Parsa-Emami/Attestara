# ADR 0002 — Attestara reference decision

**Status:** Accepted for v1

## Decision
Non-executing scanners run in ephemeral rootless Podman containers: read-only rootfs, dropped caps, no-new-privileges, seccomp, quotas, network disabled unless scanner adapter declares a reviewed requirement.

## Consequences
Implementation and tests must preserve this boundary. An alternative is allowed only through the relevant port/adapter when it preserves authority, public contracts, security invariants and migration semantics.

## Revisit trigger
Measured benchmark, security evidence, maintenance/licensing change or operational incident demonstrates the reference choice no longer satisfies the declared workload/threat model. Revisit via a new superseding ADR; never silently rewrite history.
