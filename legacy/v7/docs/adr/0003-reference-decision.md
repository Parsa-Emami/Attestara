# ADR 0003 — Attestara reference decision

**Status:** Accepted for v1

## Decision
Any feature that executes repository build/test code uses a Firecracker-class microVM runner; it is a distinct trust tier and cannot reuse scan-runner credentials.

## Consequences
Implementation and tests must preserve this boundary. An alternative is allowed only through the relevant port/adapter when it preserves authority, public contracts, security invariants and migration semantics.

## Revisit trigger
Measured benchmark, security evidence, maintenance/licensing change or operational incident demonstrates the reference choice no longer satisfies the declared workload/threat model. Revisit via a new superseding ADR; never silently rewrite history.
