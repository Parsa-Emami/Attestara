# C27 enabled local surface security matrix

Owner-approved scope: the existing single-operator loopback profile. This matrix
maps assertions to executable tests; actual runs, source/tool/provider identities,
fixture hashes, exits and outputs are recorded in `docs/evidence/C27.json`.
`docs/security/THREAT_MODEL.md` describes trust assumptions and residual risks.

## New C27 browser and HTTP coverage

All six tests live in `tests/browser/security.spec.ts` and consume the version-1
Apache-2.0 synthetic C27 fixtures. The real local supervisor, Go health process,
web server and Chromium are used. No identity, provider authorization or durable
acceptance service is mocked into existence.

| Surface / case | Expected observable result | Test |
| --- | --- | --- |
| Fixture integrity | Version/license and exact fixture plus reused C21 hashes match; scenario IDs are unique | C27 fixtures retain versioned synthetic provenance and exact hashes |
| JSON preview name/key, roadmap search | Hostile markup stays literal; no injected DOM, JS marker, pollution, request or dialog; unsafe nested keys reject and clear preview | C27 local preview escapes hostile names and keys without requests or prototype pollution |
| Passport metadata and warnings | Literal text and Unverified assessment; no injected DOM, JS marker or network request; clear/reload removes display | C27 hostile passport fields stay literal and unverified without sending selected bytes |
| Empty, malformed, oversized, traversal/absolute evidence path, forged trust/owner field, unsupported version | Exact safe error; previous passport article and assessment disappear | C27 invalid passports discard the previous display and fail with safe errors |
| Cancel a pending read after a previous valid passport | Prior display disappears; resumed native callbacks cannot restore cancelled claims; another valid read still works | C27 cancelled native file callbacks cannot restore old passport claims |
| Forged identity/role/owner/provider headers | Disabled object GET is 503; object/provider/health POST is 405 with GET/HEAD allowlist; real health remains read-only process health | C27 forged headers cannot open disabled object/provider APIs or turn health into a writer |

Request monitoring after startup covers browser requests, including GET
exfiltration, not only uploads. It permits only Chromium's automatic exact
same-origin `/icon.svg` GET without query or body on hash navigation. DOM
assertions prevent CSP from masking unsafe
HTML insertion. HTTP status checks assert method/path denial only: the web server
has no sensitive write handler or storage/provider admission route to invoke.

## Existing engine, CLI, provider and storage regressions rerun with C27

| Boundary | Executable regression coverage |
| --- | --- |
| Go listen/method admission; no source arguments | `TestHealthContractAndAdmission`, `TestVersionAndInvalidLaunch`, `TestRealListenerShutdownAndOccupiedPort` |
| Web methods, headers, asset/traversal/symlink isolation; invalid provider health URL | `tests/web/foundation.test.mjs` and `tests/web/local-launch.test.mjs` |
| Actual CLI evaluate/verify/export, bad digest/input/version and inert text | `TestC20CLICommandsAndExitCodes`; C20 remains digest-only and Unverified |
| Confined file reads, unsafe ZIPs, missing/modified bytes, inert source | C08/C09 tests; `TestC23ConfinesSourcePaths`, `TestC23RejectsBadSourcesWithoutPublishing`, `TestC23SourceStaysInert`, `TestC23CLIExportModes` |
| Local fixed-policy OPA and invalid/unavailable provider | `TestC15RealOPAAndBindings`, `TestC15RejectsInvalidInputs`, `TestC15ProviderFailuresAndTimeout`, `TestC15NoSourceExecutionAndEnvironment`, `TestOPAProcessBoundsAndEnvironment`; record the actual pinned binary, never infer provider success from skips |
| Read-only scanner/SBOM data, no URL/context fetching | C10/C11/C12 real adapter tests, including `TestC10NeverLoadsDocumentURLs`, `TestC11NeverFetchesContextOrMetadata` |
| Forgery and signed incomplete bundle | `TestC26BundleTrustMatrix`, `TestC26SignedArchiveSourceStaysInert`; real byte/signature checks, no public trust API |
| Writer isolation, crash/reopen, operation replay, quota/ENOSPC | Existing C05/C14 real storage suites; no new release authority or physical power-loss proof |
| Public metadata privacy | C25 redaction tests and existing browser redaction rendering test; arbitrary unmarked secret detection remains unsupported |

## Authorization obligations that are not enabled

| Required abuse case | Current disposition |
| --- | --- |
| Missing / expired / replayed identity | Unsupported: no identity/session or accepted release-identity state exists. No authentication or anti-replay pass is claimed. |
| Unrelated owner / object scope | Unsupported: no user/object API; owner metadata does not grant authority. Forged-header HTTP denials do not prove object ownership enforcement. |
| Widened provider permission / unavailable identity provider | Unsupported: no identity-to-provider permission surface. C15 checks a local operator-selected executable, not delegated user permission. |
| No user source execution | Tested on current browser/CLI/archive/provider paths; actual source-like sentinels remain inert. The fixed trusted local OPA program is the explicit exception for policy evaluation, not uploaded source execution. |

Private/multi-user/remote-sensitive profiles are blocked pending an explicit
authorization contract and genuine tests. C32 network/private-ingress hardening
must retain that gate. C28 performance, C29 operational readiness and C30 full
workflow remain separate unmet obligations; this matrix does not certify them.

## Commands and findings

Run the pinned toolchain: `npm run test:browser`, `npm run check`,
`go test -count=1 -v ./...`, `go vet ./...`, `python3 scripts/validate_repo.py`
and `git diff --check`. Go tests are already available since C03/C06 despite the
old C27 planning label. There is no dedicated npm lint script; report TypeScript
typecheck and Go vet accurately. Existing optional real OPA regressions require
the explicitly selected C02-pinned binary.

The existing TypeScript configuration checks `apps/web/src` only. Playwright
transpiles and executes the browser specs; a separate spec-only tsc invocation
cannot resolve Node types because this workspace has no `@types/node`. Record
that optional check as unavailable rather than adding a dependency or claiming
that application typecheck covers the spec.

The C27 added scenarios pass against existing production behavior; no production
fix was required by those scenarios. This is regression/abuse coverage, not a
claim of an exhaustive security audit. Rejected input cannot be called accepted,
and unsupported authorization cases cannot be counted as passing tests.
