# C31 implementation and validation report

C31 is verified locally for the approved Linux x64 `local-cli-snapshots` profile.
The full artifact includes the actual engine/web UI, offline resources, complete
product source/build recipes, CycloneDX 1.7 SBOM, upstream license texts, SHA-256
checksums and unsigned provenance. No domain API or database format changed.

## Observed results

- Two separately installed/build paths produced identical output bytes.
- The source archive rebuilt identical outputs without `.git`.
- Packaged CLI evaluate/verify/export, independent signatures, negative inputs,
  immutable replay/conflict, real service restart, backup/new-store restore and
  browser parity passed. The launcher had no npm/Go on PATH.
- Real SIGKILL before publication left no partial public artifact; `/dev/full`,
  existing-destination, altered-source, symlink and executable-mode checks passed.
- Existing real C05/C14 crash, ACK, quota and ENOSPC regressions passed.
- Six C31 Node regressions passed, along with the existing suite. Full npm audit
  reported zero known vulnerabilities. All 18 browser tests passed.
- Actual contracts CI commands (repository validator, engineering Python tests,
  historical and exact candidate source verification) ran in the clean candidate.
- Web-only packaging retains its existing standalone service and artifact layout.
- npm check/typecheck/build/test, Go test/build/vet/discovery, real pinned OPA,
  clone-to-value, packaging, Python planning, Node/Bash syntax, systemd unit
  verification and diff checks passed. See C31.json for exact commands and logs.

## Changed files

No files were deleted. All created/edited paths, including evidence logs:

- `.github/workflows/web.yml`
- `THIRD_PARTY_NOTICES.md`
- `apps/web/src/generated/project.json`
- `docs/ROADMAP_FA.md`
- `docs/agents/SESSION_HANDOFF.json`
- `docs/engineering/WEB_RUNBOOK.md`
- `docs/engineering/toolchain-lock.json`
- `docs/evidence/C31-audit-full.log`
- `docs/evidence/C31-audit.log`
- `docs/evidence/C31-browser.log`
- `docs/evidence/C31-check.log`
- `docs/evidence/C31-clone.log`
- `docs/evidence/C31-diff.log`
- `docs/evidence/C31-discovery.log`
- `docs/evidence/C31-go-build.log`
- `docs/evidence/C31-go-test.log`
- `docs/evidence/C31-go-vet.log`
- `docs/evidence/C31-node-syntax.log`
- `docs/evidence/C31-opa.log`
- `docs/evidence/C31-package-web.log`
- `docs/evidence/C31-package.log`
- `docs/evidence/C31-plan.log`
- `docs/evidence/C31-python-plan.log`
- `docs/evidence/C31-report.md`
- `docs/evidence/C31-resources.log`
- `docs/evidence/C31-runner-syntax.log`
- `docs/evidence/C31-source-syntax.log`
- `docs/evidence/C31-syntax.log`
- `docs/evidence/C31-systemd.log`
- `docs/evidence/C31-typecheck.log`
- `docs/evidence/C31.json`
- `docs/release/ARTIFACTS.md`
- `ops/web/nginx.conf.template`
- `ops/web/service.systemd`
- `package.json`
- `plans/progress.json`
- `plans/project-state.json`
- `plans/requirements.json`
- `scripts/package-release.mjs`
- `scripts/package-source.mjs`
- `scripts/package-web.mjs`
- `scripts/package.sh`
- `scripts/test-package.mjs`
- `tests/fixtures/attestara/C31/README.md`
- `tests/fixtures/attestara/C31/cases.json`
- `tests/web/package-release.test.mjs`

## Local artifacts

`/home/amirreza/Desktop/Projects/Attestara/dist/packages/529e0a9c49e4c2aace0bcd9cf359e68bdd71d8a2`

Runtime/source archives and their exact hashes are recorded in C31.json. The
commit in their names is a private validation snapshot, not a published Git tag
or an actual commit on the project branch. Final self-referential evidence and
status files are excluded from the program source identity; the source archive
manifests retain every exact included byte.

## Limitations and remaining work

- Unsigned local test-snapshot artifacts only. No tag, actual project commit, push, remote CI run, host configuration or public release was made.
- Node 24.19.0 remains an external runtime prerequisite; OPA/Syft/Grype binaries and scanner databases are not bundled. Pinned OPA was executed only for actual provider regression tests.
- Three-path reproducibility and installed CLI/browser proof share this Linux host; they are not independent-host C36 evidence or cross-platform support.
- Nginx is not installed on the test host: nginx -t and private ingress were not executed. The unchanged deny-by-default behavior is retained; C32 owns network verification.
- No standalone ESLint/Prettier command is defined. TypeScript, Go vet, Node/Bash syntax and diff checks passed. Full npm audit found zero known vulnerabilities; this is not proof of absence of all vulnerabilities.
- Clean clone proof used explicit system-installed Google Chrome after the recorded C30 regional browser CDN limitation; no claim of fresh browser download success.
- Distribution source filtering omits only optional copied skill directories lacking their own upstream LICENSE, without deleting any repository files. SOURCE_MANIFEST.json records every omission.
- Application crash/ACK/quota/ENOSPC boundaries are real existing C05/C14/C30 regressions; packaging additionally exercises actual SIGKILL, existing destinations and /dev/full write failure. No new storage format or authority is introduced.
- Source identity intentionally excludes evidence/self-referential status and generated roadmap metadata. Each archive separately binds its exact candidate commit and complete included file inventory; final evidence is written after packaging.

C32 is next. C35 owns independent release authentication and compatible upgrades;
C39/C40 own full release gating and maintainer-authorized publication.
