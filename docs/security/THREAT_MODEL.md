# Local profile threat model (C27)

This model covers the owner-approved enabled C27 profile: one local operator,
loopback web/engine processes, browser-local JSON/passport viewing and local CLI
commands/libraries. It does not enable private hosting or multi-user access.
Execution results are retained in CI run artifacts;
coverage and exclusions are in `tests/release/security-matrix.md`.

## Assets, actors and trust boundaries

Assets are operator files, inert evidence bytes, private scanner metadata, the
local C05 authoritative repository, policy outputs and separately configured
signature trust. Hashes and input owner strings do not grant identity or access.

An attacker may supply JSON, passport fields, ZIP members, paths, scanner reports
or HTTP headers. These values are untrusted even when read from disk. The OS user
chooses CLI roots, output paths and the optional OPA executable; evidence input
does not choose a provider or execute source. The local operator controls installed
programs and public asset/provider directories. A hostile process with the same OS
permissions can already read/change those files; this profile provides no sandbox
against that principal. Keep private data and executable paths owner-controlled.

| Boundary | Threat | Existing control and proof |
| --- | --- | --- |
| Selected file to browser | DOM/script injection, prototype keys, oversized/complex input | `inspectJson` rejects unsafe keys and bounds bytes/depth/count; React renders text. C27 checks DOM absence, execution markers, errors and network requests, including hostile filenames, JSON keys, passport metadata. |
| Passport claims to display | Fake trust, invalid paths, old data after rejection/cancellation | C21 shape/path checks; the viewer always labels its assessment Unverified. C27 rejects forged trust/owner fields, removes the prior display and resumes cancelled native FileReader callbacks to test stale-result rejection. |
| Browser to local HTTP | Header spoofing, unintended writes, access to private assets | Go and web CLIs restrict listening to numeric loopback; web serves an asset allowlist with realpath containment, CSP and safe response headers. Only GET/HEAD are admitted. Health is process health; remaining `/api/**` routes are disabled. C27 uses real HTTP denials, not mocked authorization. |
| Manifest/ZIP to local file reads | Traversal, symlink escape, missing/modified files, decompression abuse | C08/C09 validation, confined `os.Root` reads, byte/hash and expansion limits. C23 confines export reads and refuses publishing bad inputs or overwriting an existing destination. Existing Go regressions exercise actual files. |
| Evidence to policy/provider | Arbitrary source/Rego execution, substituted provider, unbounded child output | C14 evaluates a bounded first-party rule. C15 permits only the fixed C04 policy and pinned OPA digest/version with bounded process/output and isolated environment. Syft/Grype generation is operator-run, not browser execution. |
| Bundle to trust | Forged signature, candidate-key self-trust, signed incomplete bundle | C16 independently configured signer/key membership plus Ed25519 verification; C26 composes this with actual ZIP/file checks. These are stateless assessments, not accepted authority or browser trust. |
| Repository commit/reopen | Partial writes, corruption, concurrent writer, replay/lost ACK | C05/C14 single writer, atomic flushed publication, quotas and actual SIGKILL/reopen tests; ENOSPC is injected. This is local operation replay, not release identity/revision replay prevention. |
| Public passport projection | Recognizable credentials/local paths in text | C25 redacts matching metadata and free-text warnings before the digest. Arbitrary unmarked secrets are not detectable; existing imported passports are not rewritten. |

## Disabled authorization profiles

There is no enabled identity provider, login/session, owner/object API or
user-granted provider capability. Missing, expired or replayed identity,
unrelated-owner access, widened provider grants and unavailable identity providers
are therefore **unsupported / not tested as authorization**, never passed by a
mock or by a 405/503 response. C04 owner metadata is a claim, not an authenticated
principal. No browser request can commit/import release data or execute providers.

Private hosting, multi-user use and remote sensitive operations remain blocked
until a separate explicit identity/object/provider permission contract and real
positive/negative tests exist. C32 owns later loopback/origin/cache and optional
private-ingress work; TLS or a proxy alone does not supply this missing authority.

## Residual limits and continuation

- The viewer does not check the passport digest, referenced bytes or signer trust.
  The C20 CLI's passport digest check also does not establish bundle trust.
- OPA executable hashing and process execution have a replacement race. The
  executable path must remain operator-controlled; no complete executable
  authenticity or hostile-host isolation claim is made.
- Cancellation callbacks are tested with a gate around the native reader, not
  by assuming a timing delay. This proves stale-result handling, not physical
  device removal or every browser implementation.
- The checks are scoped abuse regressions, not an exhaustive vulnerability or
  dependency audit. Any observed critical/high defect in an enabled boundary must
  be fixed or that supported profile blocked before C27 verification.
- C28 measures overload/resource limits; C29 adds real operational readiness;
  C30 must prove the full clean-host browser/CLI workflow and restart/restore.
  C27 certifies none of those release claims, publication or remote CI.
