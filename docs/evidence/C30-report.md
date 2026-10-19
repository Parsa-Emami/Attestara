# C30 implementation and validation

Completed the owner-approved single-operator CLI plus local viewer workflow using existing manifest/result/passport/storage contracts. Added full file/SBOM validation, independent manifest signature trust, immutable snapshot persistence, safe backup/new-store restore and real storage readiness. No new runtime dependencies, database schema/migration, environment file or Dockerfile changes.

## Validation

- Node: 35 passed. Browser: 18 passed.
- Go packages, go vet, Go build, TypeScript and web build: passed.
- Full ten-step clean namespace clone/setup/start, real service restart and new-store restore: passed under 2 CPU and 2 GiB. Actual limits/peak, browser identity, tree and exact CLI outputs are in C30-clone.log.
- No standalone lint script exists. Four optional real-OPA tests skipped; no provider execution claimed.
- Review covered path confinement, file bounds/digest races, independent trust, redacted metadata binding, non-overwrite publication, cancellation and existing behavior.

## Changed files

### Modified

- `.github/workflows/web.yml`
- `START_HERE.md`
- `apps/web/src/generated/project.json`
- `cmd/attestara/main.go`
- `contracts/README.md`
- `docs/ROADMAP_FA.md`
- `docs/agents/SESSION_HANDOFF.json`
- `docs/engineering/CLONE_TO_RUN.md`
- `docs/engineering/ENGINE_LAUNCH.md`
- `docs/product/REFERENCE_WORKFLOW.md`
- `internal/report/passport.go`
- `ops/diagnostics.md`
- `plans/commits/C30.md`
- `plans/progress.json`
- `plans/project-state.json`
- `plans/release-gates.json`
- `plans/requirements.json`
- `plans/roadmap.json`
- `playwright.config.ts`
- `product/project.json`
- `scripts/clone-smoke.mjs`
- `scripts/serve-web.mjs`
- `scripts/start-local.mjs`
- `scripts/verify-engine-tests.mjs`
- `scripts/verify-reference.mjs`
- `tests/scenarios/clone-to-value.json`
- `tests/web/engine-readiness.test.mjs`
- `tests/web/local-launch.test.mjs`

### Created

- `cmd/attestara/readiness_test.go`
- `cmd/attestara/snapshot.go`
- `docs/engineering/C30_LOCAL_WORKFLOW.md`
- `docs/evidence/C30-browser.log`
- `docs/evidence/C30-check.log`
- `docs/evidence/C30-clone.log`
- `docs/evidence/C30-diff.log`
- `docs/evidence/C30-discovery.log`
- `docs/evidence/C30-go-build.log`
- `docs/evidence/C30-go-test.log`
- `docs/evidence/C30-go-vet.log`
- `docs/evidence/C30-next.log`
- `docs/evidence/C30-plan.log`
- `docs/evidence/C30-python-plan.log`
- `docs/evidence/C30-report.md`
- `docs/evidence/C30-typecheck.log`
- `docs/evidence/C30.json`
- `docs/evidence/private-install.json`
- `internal/workflow/publish_linux_amd64.go`
- `internal/workflow/publish_other.go`
- `internal/workflow/snapshot.go`
- `internal/workflow/snapshot_test.go`
- `scripts/clone-workflow.mjs`
- `tests/browser/clone-to-value.spec.ts`
- `tests/c30_test.go`
- `tests/fixtures/attestara/C30/README.md`
- `tests/fixtures/attestara/C30/evidence.json`
- `tests/fixtures/attestara/C30/expected.json`
- `tests/fixtures/attestara/C30/invalid-signature.json`
- `tests/fixtures/attestara/C30/manifest.json`
- `tests/fixtures/attestara/C30/negative.json`
- `tests/fixtures/attestara/C30/provenance.json`
- `tests/fixtures/attestara/C30/rule.json`
- `tests/fixtures/attestara/C30/sbom.cdx.json`
- `tests/fixtures/attestara/C30/signature.json`
- `tests/fixtures/attestara/C30/subject.txt`
- `tests/fixtures/attestara/C30/trust.json`
- `tests/fixtures/attestara/C30/untrusted.json`

Deleted: none. Protected paths changed: none. No dependency lockfile changes.

## Remaining limits

- Single local OS operator; no multi-user/private authorization, automatic expiry, latest-release authority or release-order replay prevention.
- Fresh isolated Linux namespace shares the host kernel/network and installed pinned toolchain/system libraries. It is not an independent machine or C36 proof.
- Fresh Playwright CDN browser download returned HTTP 403 geographic denial. Successful isolated acceptance used explicit system-installed Google Chrome 153.0.8010.52 with recorded SHA-256; no developer browser cache was mounted.
- The constrained 2 CPU/2 GiB measurement covers this synthetic reference workflow including preparation, not arbitrary maximum-size combined inputs or representative Grype reports.
- C05 failure-injection tests exercise ENOSPC/quota/crash behavior; no physical disk was filled during the final C30 installation.
- Four optional real-OPA tests skipped; no OPA/Syft/Grype execution claimed. No standalone lint configured.
- No remote CI, commit, push, tag, packaging release or publication was performed.
- Point-in-time readiness and lifecycle diagnostics do not independently certify product evidence; runtime diagnostics retain coreReady=false.
