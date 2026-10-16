# Attestara — Final Implementation Specification

This specification freezes the v1 implementation contract. Changes that violate an invariant, authority boundary or public contract require an ADR/RFC and migration/compatibility plan.

## HTTP conventions
- Prefix `/api/v1`; JSON UTF-8; explicit content type.
- Resource IDs are opaque UUID/ULID-style identifiers and authorization is re-evaluated after lookup.
- Cursor pagination for unbounded collections.
- Mutation request/response schemas are versioned in OpenAPI; unknown security-sensitive fields are rejected where ambiguity is dangerous.
- Stable error codes follow the common error standard.

| Method | Route | Purpose | Gate |
|---|---|---|---|
| `POST` | `/sources` | Register/upload source snapshot | auth + validation + idempotency where side-effecting |
| `POST` | `/scans` | Request non-executing isolated scan | auth + validation + idempotency where side-effecting |
| `GET` | `/scans/{scan}` | Read run/evidence status | auth + validation + idempotency where side-effecting |
| `POST` | `/policies/evaluate` | Evaluate normalized evidence | auth + validation + idempotency where side-effecting |
| `GET` | `/subjects/{digest}/passport` | Artifact/source passport | auth + validation + idempotency where side-effecting |
| `POST` | `/release-candidates/{id}/decide` | Authorized release decision | auth + validation + idempotency where side-effecting |

## Compatibility
Additive compatible changes may ship in v1. Removing/renaming/changing semantics requires deprecation + migration and normally a new API major. Golden request/response fixtures run in CI.
