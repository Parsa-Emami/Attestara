# C37 installation and workflow acceptance audit

## Result and method

The owner-approved C37 audit found no reproducible product blocker in the existing Linux x86_64 single-operator CLI/snapshot/viewer profile. No runtime behavior, public API, storage format or dependency changed. C37 adds versioned audit cases, a fixture/provenance regression and this source-bound report. It does not invent customer feedback or require a fix when the existing contract passes.

The audit reused C30/C33/C35/C36 fixtures and the existing CLI, browser, clone and package harnesses. Node 24.19.0, npm 11.9.0 and Go 1.27.2 were selected per command. The optional OPA 1.21.2 artifact was downloaded from the C02-locked URL and its SHA-256 was verified before the real-provider engine tests. Core onboarding did not invoke optional providers. Actual commands, exits, versions, fixture hashes and output hashes are recorded in `docs/evidence/C37.json` and its referenced logs.

## Findings and disposition

| Observation | Reproduction and disposition | Regression evidence |
| --- | --- | --- |
| Default local npm/Go differ from the project prerequisites | Planning preflight observed npm 11.17.0 and a default Go toolchain failing with `GOSUMDB=off`. Selecting the pinned tools and command-local Go settings resolved the test invocation; no global configuration or product change was needed. | Real `npm run setup`, `npm run check` and uncached Go tests passed. Missing/incompatible prerequisite cases still fail explicitly. |
| Failed setup removes the previous engine executable | A temporary-directory reproduction confirmed removal after a failed Go prerequisite. C03 explicitly documents and tests invalidation of stale output. Preserve this rule and rerun setup after correcting prerequisites; do not treat old output as a successful rebuild. | Existing C03 setup failure/partial-build/probe regressions passed. |
| C36 previously rejected the C34 scenario used by C35 | This recorded failure was already fixed in `2fd255ad55d7461352c42608b1cbe0e3693d59b5`. The shared validator continues to accept the historical scenario without the later onboarding section and reject malformed current onboarding data. | Validator regressions and actual installed C34/candidate upgrade/rollback passed. |
| Current installation and core workflow | No new blocker reproduced. Test failure output intentionally produced by negative cases is not a failed test or successful placeholder. | Fresh transport clone/setup/start, literal guide commands, CLI/viewer parity, restart, backup and new-store restore passed. |

## Observed validation

- `npm run check`: 62 Node tests passed, with zero failures/skips; TypeScript and production web build passed.
- `go test -count=1 -v ./...`: 118 top-level tests passed, with zero failures/skips, including real pinned OPA, missing-provider, storage crash/ENOSPC, replay and lost-ACK regressions. Engine test discovery, Go build and Go vet passed.
- Chromium: all 25 browser tests passed, including C33 keyboard workflows at 360px/1440px, invalid/empty/interrupted reads, engine outage and unverified-claim boundaries.
- Literal public-guide CLI/restart/restore, real isolated Nginx, Python planning/contracts/engineering checks, changed-test syntax and diff checks passed. No standalone JavaScript lint command exists.
- The clean local bubblewrap clone passed the existing C36 harness and C30 workflow. The package harness passed two-path and source-archive reproduction, real installed workflow, restart/restore, signed test-artifact checks and C34/candidate upgrade/rollback retaining newer accepted data. Web-only packaging passed.
- `npm audit --json` reported zero known advisories for the locked dependency graph at execution time. This is not a vulnerability-free certification.

Runtime validation used source identity `sha256:4628db4f0f57b6bab5eae68b56f39d98435e067525108f29c73048f9d4026dc1` (631 source inputs). The clone and package candidate commits/trees are recorded separately in C37 evidence. This report, the Persian overview and evidence/status/handoff updates were written after runtime validation; they do not relabel those executions as tests of a later tree. Final documentation/status consistency is checked separately.

## Remaining boundaries and recovery

There are no unresolved blockers within the audited scope. Local command equivalents cover the four existing GitHub workflows; a new remote Actions run, compiler-bootstrap/setup/upload actions, human usability study, other platforms and a new constrained-capacity measurement are not claimed. Linux headless Chrome and local bubblewrap share this host. Test signing identities are synthetic; freshness and expected source remain operator inputs, and passport authenticity remains independently unverified.

C38 retains maintenance/data policy, C39 retains the full release gate and C40 retains maintainer-authorized publication and installed published-artifact proof. To undo this audit, revert only its test/fixture/documentation/status changes. No data migration or accepted-state overwrite is required; retain newer store bytes and use the existing new-store restore procedure.
