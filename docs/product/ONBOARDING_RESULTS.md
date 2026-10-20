# Independent onboarding results

## Method

C36 runs the public Linux local workflow in a fresh Git transport clone on the existing GitHub Actions `ubuntu-latest` runner. It starts with fresh HOME, npm, Go and Playwright caches; checks the pinned Node/npm/Go versions; runs `npm run setup`; executes the literal CLI commands from `docs/release/USER_GUIDE.md`; then runs the supervised C30 CLI, restart, new-store restore and browser viewer checks from `tests/scenarios/clone-to-value.json`.

The run emits `dist/onboarding/C36-run.json`. It records the source commit, candidate tree and source identity, runner and toolchain versions, fixture/document hashes, elapsed time per phase, expected CLI exit codes and any failed phase. GitHub Actions retains this file as the `c36-onboarding-<sha>` artifact even when the onboarding step fails. The test runs no user source and does not require OPA, Syft or Grype.

## Observed result

**Local harness passed; independent-runner result pending.** On 2026-10-20, a fresh Git transport clone completed the workflow under local bubblewrap in about 22 seconds of measured onboarding phases. The toolchain was Node 24.19.0, npm 11.9.0, Go 1.27.2 and Linux x64. Setup, the literal guide test, supervised CLI, service restart, new-store restore and browser viewer passed. The run report records the source identity and fixture hashes. The browser used the installed system Chrome 153.0.8010.52, so the local run did not exercise a fresh Playwright browser download. No failures were observed in this passing run.

This local result checks the harness and product workflow but shares the host kernel; it is not independent-runner evidence. Until a fresh GitHub Actions run for this source is reviewed, C36 and N36 remain unverified and this document makes no independent-onboarding or OSS usability claim.

After the hosted run, record its run ID and attempt, exact source commit and source identity, result artifact, total and phase durations, failed steps (or none observed), and any operator-reported friction here. Do not convert a local run, a planned workflow, or an unavailable artifact into a successful C36 result.
