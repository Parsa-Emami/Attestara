# C40 synthetic publication scenarios

Repository-authored Apache-2.0 inputs for the release orchestration tests. Unit
tests generate temporary Ed25519 keys and inert archive labels, never execute
these labels, and fake GitHub only at the network boundary. No test key is the
maintainer identity. Local integration uses actual packages and records its
rehearsal status separately from public installation. C35 owns signature format
negatives and C05/C31/C35 own real crash/quota/recovery tests; reuse those suites.
