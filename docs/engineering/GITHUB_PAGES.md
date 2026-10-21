# GitHub Pages viewer

The [public viewer](https://parsa-emami.github.io/Attestara/#passport) hosts the
browser-local passport display and JSON structure preview at `/Attestara/`.
Selected files stay in tab memory. It does not run Go, evaluate policy, grant
signer trust or persist/restore snapshots. Use [the local application](CLONE_TO_RUN.md)
for the complete CLI workflow.

Repository Settings → Pages uses GitHub Actions with HTTPS. The Pages workflow
builds and tests each main push, then deploys only that tested static artifact
through the `github-pages` Environment. It never receives the signing Secret.

```sh
npm ci --ignore-scripts
node scripts/build-pages.mjs
npx --no-install playwright install chromium
node scripts/test-pages.mjs
```

The builder writes `dist/pages`, uses the repository base path and relative
manifest/icons, and retains direct `#passport` navigation. A CSP meta policy
restricts requests. No service worker or offline-install guarantee is provided.
The example passport is licensed synthetic data, not a verified customer report.

The real browser smoke test covers import/clear, JSON preview, direct navigation,
narrow reflow and absence of outbound selected bytes, API requests or script
errors. Reports/screenshots are under `dist/pages-check` and CI run artifacts.
`deployment.json` records source commit/dirty state and `engine_hosted: false`;
website deployment does not certify a signed runtime release.

For rollback, revert the website source change and deploy the reviewed main
commit. The site stores no user snapshots. The release Environment and signing
workflow remain independent. Action source pins are retained in the workflow.
