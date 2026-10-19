# Continue Attestara

Current: existing local CLI and browser passport viewer, real manifest/SBOM/file validation, independent signature assessment and recoverable immutable snapshots with backup/new-store restore. C30 certification is source-bound in docs/evidence/C30.json; private/multi-user and latest-release authority are unsupported.

Use the pinned Linux x86_64 Node/npm/Go prerequisites in docs/engineering/CLONE_TO_RUN.md.

```sh
npm run setup
npm start
```

Open http://127.0.0.1:4173. Read docs/engineering/CLONE_TO_RUN.md for actual prerequisites/capability. For implementation read AGENTS.md, docs/agents/EXECUTION_CONTRACT.md and ROADMAP.md, then use:

```sh
npm run validate:plan
npm run plan -- status
npm run plan -- next
```

Each task links exact work/resources/tests. C30 is first actual local usability and C40 full OSS v1; both require evidence. Do not run historical bootstrap or modify protected files.

For the approved CLI-to-viewer reference workflow and safe recovery, read [C30 local workflow](docs/engineering/C30_LOCAL_WORKFLOW.md).
