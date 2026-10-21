# Install and run

Supported: Linux x86_64, one local OS operator. Install Git, Node 24.19.0,
npm 11.9.0 and Go 1.27.2 on PATH. Use [RESOURCE_LOCK.md](RESOURCE_LOCK.md) for
the checksum-pinned Go source-build route when the prebuilt archive is unavailable.

```sh
git clone https://github.com/Parsa-Emami/Attestara.git
cd Attestara
npm run setup
npm start
```

For a downloaded source ZIP, extract it into a new directory, enter the extracted
project and run the same `npm run setup` / `npm start` commands; setup does not
require Git metadata. Dependency preparation requires public package downloads
or populated caches. Application operations run locally.

Open http://127.0.0.1:4173/#passport. No database, Docker, cloud account or public
domain is needed. Setup installs the locked npm dependencies with lifecycle
scripts disabled, builds the web assets and Go engine, and stops on any failure.
Ctrl+C stops the supervised web/engine processes. Select another port with
`npm start -- --port 4301`; Apache can continue using port 80.

`npm run setup -- --check` checks the pinned npm prerequisite without building.
`npm run setup -- --skip-install` builds with already installed dependencies.
`npm run doctor` probes the actual built web assets and engine executable.

The browser previews JSON structure and displays passport claims without upload.
It does not validate referenced files, passport hashes or signer trust. Use
[the user guide](../release/USER_GUIDE.md) and [CLI reference](LOCAL_WORKFLOW.md)
for actual evaluate/verify/export/restore operations.

The default store is `.local-data/attestara`; select an external private absolute
store path for installed releases. Keep backups separate. The file store supports
one writer, not shared or multi-user access. Missing optional OPA/Syft/Grype tools
do not block the core workflow.

The launcher accepts loopback only. Domain HTTP APIs remain disabled. See
[the network boundary](../../infra/deployment.md) and
[web runbook](WEB_RUNBOOK.md) for optional operator-reviewed service/proxy templates.
Other operating systems, hosted sensitive workloads and multi-user authorization
are outside this supported profile. The 2 vCPU / 2 GiB resource target is not a
full-size or scanner-workload capacity guarantee.

Run checks from [ENGINE_TESTS.md](ENGINE_TESTS.md) when developing or deploying
changed source. A source ZIP can run setup/start; provenance-sensitive packaging
and clean-clone checks additionally require a committed Git checkout.
