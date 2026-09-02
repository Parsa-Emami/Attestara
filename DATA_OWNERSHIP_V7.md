# Attestara — Data Ownership v7

- **PostgreSQL:** repos/runs/evidence metadata/policies/decisions
- **ObjectStore:** raw scanner outputs, SBOM/VEX/attestations
- **Valkey:** queues only
- **Runner FS:** ephemeral, never authority

## Universal rules
- cache is never authority
- derived projections have checkpoint + rebuild + staleness metric
- secrets/evidence follow project-specific retention and redaction
- deletion does not rewrite immutable audit/custody history
