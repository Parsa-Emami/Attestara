# Attestara

[![Source](https://github.com/Parsa-Emami/Attestara/actions/workflows/contracts.yml/badge.svg?branch=main)](https://github.com/Parsa-Emami/Attestara/actions/workflows/contracts.yml?query=branch%3Amain) [![Plan](https://github.com/Parsa-Emami/Attestara/actions/workflows/plan.yml/badge.svg?branch=main)](https://github.com/Parsa-Emami/Attestara/actions/workflows/plan.yml?query=branch%3Amain) [![Web](https://github.com/Parsa-Emami/Attestara/actions/workflows/web.yml/badge.svg?branch=main)](https://github.com/Parsa-Emami/Attestara/actions/workflows/web.yml?query=branch%3Amain)

[CI source integrity and dependency updates](docs/engineering/CI_SOURCE_INTEGRITY.md)


Open-source local-first application under [Apache-2.0](LICENSE). The current
Linux x86_64 single-operator profile evaluates local evidence with the Go CLI,
verifies referenced bytes and independently configured signatures, preserves
immutable snapshots and opens portable passports in a browser without upload.
Policy, byte integrity and signer trust are separate assessments; the browser
displays unverified claims. A published stable OSS v1 remains pending.

## Clone and run the current workspace

Prerequisites: Linux x86_64, Git, Node 24.19.0, npm 11.9.0 and Go 1.27.2 on PATH. See the pinned [Go source-build route](docs/engineering/RESOURCE_LOCK.md).

```sh
git clone https://github.com/Parsa-Emami/Attestara.git
cd Attestara
npm run setup
npm start
```

Open http://127.0.0.1:4173. Follow the [user guide](docs/release/USER_GUIDE.md)
to evaluate the synthetic example, view its passport and verify/restore a
snapshot. No Python, database, paid/cloud account, domain or sibling clone is
needed to run this profile. Domain HTTP APIs and multi-user authorization remain
unavailable. [Clone guide](docs/engineering/CLONE_TO_RUN.md).

Apache can keep port 80: the application uses 4173 (or select another port with
`npm start -- --port 4301`). For an optional isolated Nginx proxy on loopback
8080, see [the web runbook](docs/engineering/WEB_RUNBOOK.md#test-alongside-apache-on-port-80).

## Full product endpoint

Load valid sample SBOM with manifest -> evaluate local policy -> produce passport -> change one byte -> verify failure -> distinguish unsigned from signed/trusted.

[C30 evidence](docs/evidence/C30.json) records the local workflow including
restart/restore; [C33 evidence](docs/evidence/C33.json) records browser and
keyboard checks at 360/1440px. Those historical checks retain their original
source identities and limits. [C40](plans/commits/C40.md) remains the published
OSS v1 gate. [Runtime resources](ops/runtime-requirements.json).

## Support and security reports

Use [Issues](https://github.com/Parsa-Emami/Attestara/issues) for non-sensitive
questions/bugs with the commit, tool versions, command and redacted expected/actual
result. Use [SECURITY.md](SECURITY.md) for private vulnerabilities and its live
activation status. Reports and responses are intended to remain in GitHub;
no dedicated security mailbox is required.

## Implement the next commit

```sh
npm run plan -- status
npm run plan -- next
npm run plan -- show C07
npm run check
```

Each [work card](plans/COMMIT_ROADMAP.md) specifies paths, actions, resources, fixtures, acceptance, negative tests, command availability and rollback. [AGENTS.md](AGENTS.md) and [execution contract](docs/agents/EXECUTION_CONTRACT.md) let an agent resume from this repository alone. [فارسی](docs/ROADMAP_FA.md).

[Open-source contract](OPEN_SOURCE.md) · [Contributing](CONTRIBUTING.md) · [Third-party inventory](THIRD_PARTY_NOTICES.md) · [Definition of done](docs/release/DEFINITION_OF_DONE.md) · [Security](SECURITY.md). No DB schema, migration, .env or Dockerfile changes are authorized.
