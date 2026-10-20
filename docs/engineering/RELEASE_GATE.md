# C39 release readiness gate

Run `npm run release:check` through Node 24.19.0, npm 11.9.0 and Go 1.27.2
on PATH, from a clean committed checkout with full Git history. Uncommitted
implementation work can be tested in an isolated clean candidate checkout using
the same temporary-index/commit-tree pattern as the package tests. This does not
move the developer's branch or original index.

The command always executes the checks. It never reads an earlier report as a
substitute for a new execution. Exit 0 means this run passed; 2 means readiness
was not demonstrated; 1 means the run was operationally unavailable/interrupted.
The full gate is intentionally slower than `validate:plan`.

## Prerequisites and execution

Use the existing Linux clone, browser and packaging prerequisites, including
Python 3, Git, GNU tar/gzip/coreutils, Nginx for the isolated proxy test and
Chromium/Chrome. Install the pinned Playwright browser with
`npx --no-install playwright install --with-deps chromium`, or select the explicit
installed system browser using `WEB_TEST_CHROMIUM`. Local clean-clone validation
uses bubblewrap; GitHub uses its fresh hosted runner. No existing service is stopped.

The gate verifies the pinned OPA binary selected by `ATTESTARA_TEST_OPA`, or
downloads and checksum-verifies it into this run's ignored directory. OPA is used
for real adapter regressions; it remains optional for the core product. No Syft
or Grype scanner is executed. Network access is needed for setup/clone/build,
dependency advisory checks and the live public security-reporting setting.
Unavailable required checks fail explicitly; they never mean zero vulnerabilities.

The fixed commands cover repository/plan/source integrity, historical evidence,
Python and Node tests, web TypeScript/build, Go discovery/build/vet/uncached JSON
tests with real OPA, documented CLI and Nginx, browser security/accessibility,
clean-clone restart/restore, source/runtime package reproduction, installed
upgrade/rollback, advisory/reporting checks and syntax/diff checks. Go proof needs
package start/completion and ordered run/pass events for the existing required
anchors. Packages explicitly reporting no test files are distinguished from
skipped tests. No required test may be skipped. There is no standalone JavaScript
lint command; the actual vet/syntax/diff checks are reported without a lint claim.

## Source and historical evidence

The exact C01-C38 set, evidence hashes, historical source commits and compatibility
records are reviewed in `plans/release-gates.json`. Every task must be verified
with real non-template proof and successful command records. Missing, modified,
duplicate, unsafe or wrong-source evidence and corrupted referenced output fail.
Recorded command descriptions are data and are never executed.

Historical scope stays attached to its original source. Three existing hash
framings are reproduced explicitly: raw digests, hexadecimal digests plus LF,
and raw digests plus LF. C07 retains its original absolute path labels for hashing
while resolving only the fixed old prefix through the historical Git tree.
C32's actual digest uses hex/LF despite its original raw-byte description.
C36 binds the actual hosted runner commit; its recorded output digest identifies
the GitHub ZIP container. The original ZIP and JSON member are retained separately
with independent checksums. C37's measured source is reconstructed using the two
documented post-runtime documentation changes. Historical C24-C27 deferred work
is explicitly resolved against verified C30/C38 and the approved profile; private
identity, automatic expiry and latest-release authority remain unsupported.

C31's typecheck log omitted its last LF when retained. The C39 revalidation on the
original C31 commit reproduces the exact originally recorded output digest. Only
that explicit, source-bound recovery is accepted; arbitrary newline changes or
rehashed corrupt logs are not normalized into success. Original task records are
preserved. See the C39 supplementary evidence for the original invocation/output.

Fresh source inputs use the C36 runner's sorted path/NUL/raw-SHA256/LF identity.
Task evidence, progress/requirements/project-state/handoff, generated web status
and historical SHA256SUMS are excluded from that runtime identity, as in C36.
Workflows, tests, fixtures, locks and implementation documentation are included.
The exact initial checkout is independently checked by `verify_ci_source.py`.
Input bytes/modes must still match after testing. Temporary package/clone commit
IDs are not accepted without their measured source identity/tree matching the
current tested source.

## Reports and completion

Each invocation creates a new `dist/release-gate/run-*/` directory. Its report
contains actual checkout commit/tree, source inventory, tool identities, fixture
hashes, command exits and output hashes, integration results and blockers.
Failed runs retain their own report; interruption cannot reuse earlier success.
The directory may contain a downloaded OPA test prerequisite; CI uploads reports,
logs and source metadata, not that binary. Logs/artifacts are diagnostic evidence,
not a promise of power-loss durability or an authenticated maintainer release.

The read-only GitHub workflow runs for PRs, main pushes and manual dispatch. It
retains the current run's evidence and tested package artifacts for 14 days.
The reviewed C39 attestation and `docs/evidence/v1-gate.json` retain source-bound
results and distinguish local execution from actual candidate GitHub runs.
Changing evidence/status after runtime checks does not relabel older runs.

C39 certifies the supported release candidate only after required local and real
GitHub results are verified. Publication, a maintainer-controlled signing identity,
and installation of actually published artifacts remain C40. No schema, migration,
environment file, Dockerfile, domain API or product data format changes are needed.
