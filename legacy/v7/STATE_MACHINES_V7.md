# Attestara — State Machines v7

- **Scan:** `CREATED -> QUEUED -> PREPARING -> RUNNING -> NORMALIZING -> COMPLETE | FAILED | CANCELLED`
- **ReleaseGate:** `PENDING -> EVALUATING -> PASS | FAIL | EXCEPTION`
- **Runner:** `ALLOCATING -> ACTIVE -> DESTROYING -> DESTROYED | QUARANTINED`

Transitions are domain commands with authorization, optimistic version check where applicable, audit event and invariant tests. Illegal transitions fail closed and are observable.
