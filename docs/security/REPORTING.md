# Private vulnerability reporting operations

Attestara uses GitHub Private Vulnerability Reporting for reports and responses.
No dedicated mailbox or external form is needed. See [SECURITY.md](../../SECURITY.md).

## Activate and verify

An owner/admin must open **Settings → Advanced Security → Private vulnerability
reporting → Enable**. This is a GitHub setting: committing a policy or changing
Nginx cannot activate it. No paid subscription or unrelated permission change is
needed. An already authenticated GitHub CLI with suitable repository permission
can enable only this feature:

```sh
gh api --method PUT repos/Parsa-Emami/Attestara/private-vulnerability-reporting
npm run security:check
```

The PUT should return HTTP 204. The separate public GET must print
`enabled: true` and exit 0. Exit 2 is disabled; exit 1 is unavailable/invalid,
including API limits or network failure. The checker needs no token, submits no
report, rejects redirects and has a 10-second request timeout. It is an online
operator check, separate from the offline test suite.

Then open **Security → Advisories → Report a vulnerability** and verify that
the private form is available. Do not submit a dummy report or publish an
advisory to test the route. Record actual UTC time/API response in evidence,
then update the activation notice in SECURITY.md. A link or unit-test fixture alone
cannot certify the live setting. Keep credentials out of source and evidence.

## Handle reports

Manage reports in [Security → Advisories](https://github.com/Parsa-Emami/Attestara/security/advisories).
The researcher and authorized maintainers collaborate in the private advisory.
Follow the acknowledgement/assessment targets in SECURITY.md, request minimal
redacted reproductions and coordinate a patch/disclosure. Never copy private
details to public Issues, PR comments, commit messages or evidence.

## Confirmed activation

The owner enabled **Private vulnerability reporting** in this repository's
Advanced Security settings on 2026-10-19. GitHub showed **Repository settings
saved** and **Disable private vulnerability reporting**. The independent public
API check then returned HTTP 200 with `{"enabled":true}` and exit 0.
The private advisory form and its access/visibility notice were inspected in
the signed-in maintainer session. No dummy report, advisory or email was sent,
and notification delivery was not tested. Reports and follow-up remain in
GitHub; no personal or dedicated mailbox is published.

Official references: [configure reporting](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository),
[report privately](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/report-privately),
[REST read/enable endpoints](https://docs.github.com/en/rest/repos/repos#check-if-private-vulnerability-reporting-is-enabled-for-a-repository).
