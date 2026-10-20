# C35 implementation and validation report

Implemented the owner-approved offline Ed25519 release verifier and a real installed C34-to-candidate upgrade/software-rollback test. The external C05 store preserves both snapshots and newer accepted bytes. Existing v1 signature/trust and C31 artifact formats are reused. No dependency, database schema, migration, .env, Dockerfile, HTTP API or product UI behavior change.

## Validation

56 Node tests, 118 Go tests (real pinned OPA, zero skips), 25 browser tests, TypeScript, web build, Go build/vet/discovery, JavaScript syntax, isolated clean clone, reproducible source/runtime packaging, documented CLI and real isolated Nginx all passed. npm audit found zero known vulnerabilities. The existing GitHub workflow test/validation commands ran locally; no remote run or publication is claimed. Exact commands, outputs, source identities, fixture hashes and tool executable hashes are in C35.json.

Baseline: `f103087fe933937775f701dbc9170cdfd9ee3a7d`. Runtime candidate: `395c45f173ec7d01c241104ed2aabd6df7f1685d`. Product source identity: `sha256:bc3a3b6a724c0ffff690becc3858ce8c1aec1e9859fbd5d212da70e077ee0092`.

## Security and limits

All four required artifacts are streamed and hashed before trusted success; trust is independent of the candidate key, and expected source/freshness are explicit. Mutation, absent files, bad signatures, unknown/wrong keys, unsafe paths, malformed/oversized metadata, symlinks/FIFO and stale/unexpected sources fail. Verification never extracts or executes artifact/user bytes. Test private keys remain in memory.

No functional blocker remains for the approved C35 scope. Official maintainer identity/publication, automatic expiry/latest-version authority and independent-host onboarding are outside this proof. The source-integrity checker intentionally rejected the dirty root checkout; it passed in the clean candidate. A final packaging attempt also correctly rejected generated roadmap bytes captured during a concurrent check; sequential rerun passed after generation completed. No check was disabled.

## Documentation

ARTIFACTS.md documents detached signing/independent verification, trust bootstrap, bounds, exits, source rebuilds and data-preserving upgrade/rollback. USER_GUIDE.md links that procedure. ROADMAP_FA.md records the Persian capability overview. Planning state and session handoff now point to C36.

## Created files

- `docs/evidence/C35-audit.log`
- `docs/evidence/C35-browser.log`
- `docs/evidence/C35-check.log`
- `docs/evidence/C35-clone.log`
- `docs/evidence/C35-diff.log`
- `docs/evidence/C35-discovery.log`
- `docs/evidence/C35-engineering.log`
- `docs/evidence/C35-final-browser.log`
- `docs/evidence/C35-final-check.log`
- `docs/evidence/C35-go-build.log`
- `docs/evidence/C35-go-test.log`
- `docs/evidence/C35-go-vet.log`
- `docs/evidence/C35-guide.log`
- `docs/evidence/C35-next.log`
- `docs/evidence/C35-nginx.log`
- `docs/evidence/C35-package-confirmed.log`
- `docs/evidence/C35-package-final.log`
- `docs/evidence/C35-package-web.log`
- `docs/evidence/C35-package.log`
- `docs/evidence/C35-pins.log`
- `docs/evidence/C35-plan.log`
- `docs/evidence/C35-python-plan.log`
- `docs/evidence/C35-repo.log`
- `docs/evidence/C35-report.md`
- `docs/evidence/C35-resources.log`
- `docs/evidence/C35-setup.log`
- `docs/evidence/C35-source.log`
- `docs/evidence/C35-syntax-package.log`
- `docs/evidence/C35-syntax-runner.log`
- `docs/evidence/C35-syntax-shell.log`
- `docs/evidence/C35-syntax-unit.log`
- `docs/evidence/C35-syntax-upgrade.log`
- `docs/evidence/C35-syntax-verify.log`
- `docs/evidence/C35-typecheck.log`
- `docs/evidence/C35-verify-artifacts.log`
- `docs/evidence/C35.json`
- `docs/evidence/release-verification.json`
- `scripts/verify-release.mjs`
- `tests/fixtures/attestara/C35/README.md`
- `tests/fixtures/attestara/C35/cases.json`
- `tests/fixtures/attestara/C35/provenance.json`
- `tests/release/upgrade-recovery.mjs`
- `tests/web/release-verification.test.mjs`

## Modified files

- `.github/workflows/web.yml`
- `apps/web/src/generated/project.json`
- `docs/ROADMAP_FA.md`
- `docs/agents/SESSION_HANDOFF.json`
- `docs/release/ARTIFACTS.md`
- `docs/release/USER_GUIDE.md`
- `package.json`
- `plans/progress.json`
- `plans/project-state.json`
- `plans/requirements.json`
- `scripts/package-release.mjs`
- `scripts/test-package.mjs`

No files were deleted. Changes remain uncommitted; no push was performed.
