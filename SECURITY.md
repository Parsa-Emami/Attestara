# Security policy

## Private reports in GitHub

Use [Report a vulnerability](https://github.com/Parsa-Emami/Attestara/security/advisories/new)
under **Security → Advisories**. Reports and follow-up belong in the private
GitHub advisory conversation with authorized maintainers. No dedicated email
address is required or published. Issues, pull requests and Discussions are
public and must not contain vulnerability details.

**Activation is pending:** on 2026-10-19 the public GitHub API returned
`enabled: false`. The link above is the intended route, not proof of an active
private channel. If the button is missing, open an Issue containing only
“Please enable private vulnerability reporting”; withhold technical details
until a private route is available. See [maintainer activation and verification](docs/security/REPORTING.md).
C34 remains unverified until the live check returns `enabled: true` and its
remaining acceptance checks pass.

## What to include privately

- Affected commit SHA/artifact version and local profile.
- Impact, expected/actual behavior and minimal reproduction steps.
- Synthetic inputs, tool/platform versions and redacted diagnostics.
- Suggested mitigation, if available, and disclosure/credit preferences.

Never attach real credentials, private source, customer/incident records, live
stores or private network details. Reproduce with synthetic data and test only
systems you own or are authorized to assess. Do not execute uploaded user source.

## Response and disclosure

The maintainer aims to acknowledge within **3 business days** and provide an
initial assessment within **7 business days**. These are targets, not a fix
deadline. Continue in the same private advisory, including follow-ups if there
is no response. Coordinate remediation, credit and disclosure with the reporter
before publishing details. Each participant manages their GitHub notifications.

## Supported scope

Fixes target current `main` on Linux x86_64 for the single-operator local CLI,
immutable snapshots and browser file viewer. No stable v1 or historical release
support line is published yet. `legacy/v7` is historical. Remote/multi-user
domain APIs, identity/object authorization and hosted sensitive workloads remain
unsupported. See the [threat model](docs/security/THREAT_MODEL.md),
[network boundary](infra/deployment.md) and [release policy](docs/release/RELEASE_POLICY.md).
A policy pass, digest or displayed passport does not establish producer trust,
current-release authority, compliance or absence of vulnerabilities.
