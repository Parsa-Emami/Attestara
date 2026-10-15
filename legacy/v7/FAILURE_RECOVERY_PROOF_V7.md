# Attestara — Failure & Recovery Proof v7

## Mandatory injected failures
- zip-slip/symlink archive
- scanner hang
- disk exhaustion
- network escape attempt
- poisoned scanner output
- runner crash
- duplicate job
- forged evidence

## Pass criteria
For every failure: no invariant violation; no unauthorized bypass; no unbounded retry storm; orphan resources are reconciled; user-visible state becomes correct; telemetry identifies cause; retry/cancel/manual recovery is documented; recovery test is automated where feasible.
