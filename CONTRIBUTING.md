# Contribution workflow

For usage, see [the user guide](docs/release/USER_GUIDE.md). Ordinary support
belongs in public Issues with a commit SHA, platform/tool versions, exact
command and redacted expected/actual results. Share synthetic reproduction
inputs only. Vulnerability details, private logs and real data must follow
[SECURITY.md](SECURITY.md) and remain in the private GitHub conversation.
Maintainers can inspect the live reporting setting with `npm run security:check`;
activation requires repository admin permission as described in
[the reporting runbook](docs/security/REPORTING.md). An inactive or unavailable
private route must keep C34 unverified.

Read OPEN_SOURCE.md, AGENTS.md and ROADMAP.md. Use npm run setup and npm run plan -- next, then read that complete task card. Implement one real dependency-ready work unit with meaningful positive/negative tests. Preserve Apache-2.0, upstream attribution and DCO.md; no secrets/private data or license-unknown fixtures.

Run actual relevant engine checks and npm run check; browser changes also run test:browser and clone changes run test:clone. Record source/fixture identities, command exits and observed results, then update progress truthfully. Use focused Conventional Commits/PRs describing the concrete behavior, validation and remaining blockers. Do not disable checks, fake providers or change protected DB schema/migration/.env/Dockerfile files. Security reporting follows SECURITY.md.
