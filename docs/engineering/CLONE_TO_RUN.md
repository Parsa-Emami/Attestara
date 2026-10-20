# Attestara: clone to run

## Runnable today

On Linux x86_64, install Git, Node 24.19.0, npm 11.9.0 and Go 1.27.2 on PATH. Use the checksum-pinned official [Go source-build route](RESOURCE_LOCK.md) when the prebuilt archive is unavailable. Then:

```sh
git clone https://github.com/Parsa-Emami/Attestara.git
cd Attestara
npm run setup
npm start
```

Open http://127.0.0.1:4173. Setup installs the committed web lockfile with lifecycle scripts disabled, stops on errors, builds the workspace and prepares the real Go executable. The unified launcher displays actual engine process health/version. No database, cloud account or domain is required. The workspace previews JSON structure, browses the roadmap and opens a local C19 passport at `#passport`. The viewer renders unverified claims from a browser-selected file without uploading it; it does not check the passport digest, referenced bytes or signer trust. The approved single-operator CLI snapshot workflow is documented in C30_LOCAL_WORKFLOW.md; current acceptance status and limitations live in docs/evidence/C30.json.

`npm run setup -- --check` checks the pinned npm version and shows preparation without changing files. Setup also refuses an npm version other than `package.json`'s `packageManager` pin. `--skip-install` builds using already installed dependencies. `npm run doctor` reports actual web/core state. `npm run release:check` runs the complete source-bound checks each time; see [release-gate prerequisites and reports](RELEASE_GATE.md). Ctrl+C stops the web server and Go child. Ports can be selected with `npm start -- --port 4301` when running multiple projects.

## C32 local network boundary

The default address is `http://127.0.0.1:4173`; an owned domain is not a prerequisite. The launchers reject non-loopback binds. The browser uses same-origin `/api`, while domain API work remains disabled. Dynamic pages, configuration and health responses are not cached; fingerprinted web assets can be cached immutably. [Local network and optional ingress](../../infra/deployment.md) describes the tested boundary and operator-reviewed TLS/private CIDR template. This does not turn the local workspace into a remote multi-user service.

## Supported local profile and resources

The clone/setup/start sequence prepares and supervises the Go engine for the
supported single-operator CLI/snapshot profile. Follow [the user guide](../release/USER_GUIDE.md)
for actual synthetic evaluate/verify/view commands. C31 documents locally
verified convenience artifacts and a source-build alternative; published releases
remain pending. Downloads may be needed to prepare resources; payloads are not
sent to a compulsory proprietary service. No global runtime install, trust
change or user-data deletion occurs implicitly.

Local resources: optional local OPA and Syft/Grype report ingestion. The read-only Grype adapter rejects more than 10,000 matches before typed projection. Hardware target: 2 vCPU / 2048 MiB RAM / 10240 MiB free disk plus real dataset/provider/backup space. C28 measures the existing CLI evaluate/verify/ZIP-export workload and one-byte-over-limit rejection under a 2 vCPU / 2048 MiB cgroup with at least 10240 MiB initially free in an isolated temporary filesystem; see `docs/evidence/performance.json`. This supports only that measured CLI workload. Peak memory for representative Grype reports and the complete C30 workflow remain unmeasured; the full product hardware profile is not certified. Full engine support initially targets Linux x86_64; other hosts require their own actual C30/C33 evidence. Rootless workloads need Linux or a tested Linux/WSL2 host.

`local-demo` uses synthetic fixtures and isolated development providers; `local-private` requires real identity/permission, provider durability, revocation and restore evidence before sensitive use. A development provider mode never becomes a production vault/gateway by relabeling it.

## Checks

```sh
npm run check
go test ./...
npx --no-install playwright install chromium
npm run test:browser
npm run test:clone
npm run doctor
```

The current test:clone performs a real Git transport clone and the approved C30 CLI snapshot workflow, including negative cases, restart, backup/restore and browser passport parity. It verifies local storage readiness and disabled domain admission; it does not certify external domain APIs or optional providers. See each later C task for future engine commands and prerequisites. Optional hosting/package operations remain in WEB_RUNBOOK.md.

## Ports, support and security reports

Apache occupying port 80 does not block `npm start` on 4173. Select another
local application port with `npm start -- --port 4301`. Optional Nginx testing
on loopback 8080 uses the isolated config in [WEB_RUNBOOK.md](WEB_RUNBOOK.md#test-alongside-apache-on-port-80);
its upstream port must match the launcher. No existing Apache service or global
Nginx site needs changing.

Public Issues are for non-sensitive support. [SECURITY.md](../../SECURITY.md)
defines the enabled private GitHub route, supported scope and response targets.
`npm run security:check` reads the live setting without a token:
exit 0 is enabled, 2 disabled, 1 unavailable. It is an online operator check,
not an offline build prerequisite, and submits no report.

## C30 approved local snapshot composition

The current CLI snapshot workflow, full file/signature verification, real storage readiness and safe backup/restore are described in [C30 local workflow](C30_LOCAL_WORKFLOW.md). The earlier C03/C20/C29 sections above retain their historical capability boundaries. Full C30 status is determined by source-bound evidence, not these instructions.

C30 test-only prerequisites: locally, bubblewrap; on GitHub Actions, its ephemeral runner VM; both need a fresh Playwright browser download or an explicit system browser via `WEB_TEST_CHROMIUM`. These are acceptance-test tools, not required to run the CLI or local viewer.
