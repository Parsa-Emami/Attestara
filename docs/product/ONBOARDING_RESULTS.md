# Independent onboarding results

## Method

C36 runs the public Linux local workflow in a fresh Git transport clone on the existing GitHub Actions `ubuntu-latest` runner. It starts with fresh HOME, npm, Go and Playwright caches; checks the pinned Node/npm/Go versions; runs `npm run setup`; executes the literal CLI commands from `docs/release/USER_GUIDE.md`; then runs the supervised C30 CLI, restart, new-store restore and browser viewer checks from `tests/scenarios/clone-to-value.json`.

The run emits `dist/onboarding/C36-run.json`. It records the source commit, candidate tree and source identity, runner and toolchain versions, fixture/document hashes, elapsed time per phase, expected CLI exit codes and any failed phase. GitHub Actions retains this file as the `c36-onboarding-<sha>` artifact even when the onboarding step fails. The test runs no user source and does not require OPA, Syft or Grype.

## Observed result

**Fresh hosted-runner workflow passed.** GitHub Actions run [39389710759, attempt 1](https://github.com/Parsa-Emami/Attestara/actions/runs/39389710759) completed on `ubuntu24` for source commit `2fd255ad55d7461352c42608b1cbe0e3693d59b5`, source identity `sha256:2f62e1e1b033d493e1fc597bd33dd05dca764f8b6721929ffc3ea14361625632` (628 files). The C36 result is retained as artifact [c36-onboarding-2fd255ad55d7461352c42608b1cbe0e3693d59b5](https://github.com/Parsa-Emami/Attestara/actions/runs/39389710759/artifacts/12249660279), SHA-256 `d6ae822d794916a87ce6d047413e997154afcb4abe81dc34325362233265e3b5`.

The runner used Node 24.19.0, npm 11.9.0, Go 1.27.2 and Git 2.55.0 on Linux x64 (Ubuntu 24 image). The measured onboarding phases totaled 18,992 ms. Setup, literal public-guide commands, CLI workflow, service restart, new-store restore and browser viewer passed; no failures or limitations were recorded. The run confirms `restart_preserved`, `new_store_restore` and `browser_viewer` as successful, and `user_source_execution` as false. Optional providers were not invoked and were not required for this base profile.

The first hosted attempt for the prior implementation commit failed because the shared C30 workflow validator required the C36-only onboarding section when C35 exercised its historical C34 baseline. The regression was fixed in `scripts/clone-workflow.mjs`, with coverage that keeps the C34 scenario compatible while rejecting malformed C36 onboarding data. The subsequent fresh runner execution above passed. Expected failure output from negative package-test cases is part of those assertions and is not a failing workflow step.

The local bubblewrap run remains harness evidence only. C36/N36 evidence applies to the recorded 2fd source commit; this report and the planning/evidence status updates are post-runtime attestation and do not change that tested source identity. No human usability study, optional-provider execution, publication or C40 release claim is made.
