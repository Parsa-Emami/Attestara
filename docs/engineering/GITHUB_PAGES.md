# GitHub Pages viewer

GitHub Pages hosts the existing browser-local passport viewer, bounded JSON
structure preview and roadmap at the repository subpath `/Attestara/`. Selected
files stay in the tab. The site does not run Go, evaluate policy, grant signer
trust or persist/restore snapshots. Use [the full local application](CLONE_TO_RUN.md)
for that workflow. Publishing this site does not verify the signed C40 release.

The owner explicitly requested this public Pages deployment. Repository Settings
→ Pages uses **GitHub Actions**, with HTTPS enforced on the default GitHub domain.
`.github/workflows/pages.yml` builds and tests the static site on each main push,
then deploys only that tested artifact through the `github-pages` Environment.
The deploy job has `pages: write` and `id-token: write`; neither job receives the
release signing Secret, and neither has repository content write access.

## Local verification

Use the pinned Node/npm prerequisites and install dependencies without scripts:

```sh
npm ci --ignore-scripts
node scripts/build-pages.mjs
npx --no-install playwright install chromium
node scripts/test-pages.mjs
```

The builder uses an explicit `github-pages` target and writes only `dist/pages`,
leaving `dist/web` and the default local runtime profile intact. Vite prefixes
assets and runtime configuration with `/Attestara/`. Manifest scope, start URL
and icon references remain relative to the project site. Hash navigation supports
direct `#passport` links without a server-side fallback. A restrictive CSP meta
policy is included; GitHub Pages does not provide the local server's custom
security, readiness or API response headers. No service worker or offline-install
guarantee is added. The bundled example is the existing Apache-2.0 C19 synthetic
passport fixture, with the same unverified viewer assessment.

The real Chromium smoke test uses a static server confined to the project
subpath. It exercises passport import/clear, JSON preview, roadmap filtering,
direct hash navigation and narrow-screen reflow. It rejects failed responses,
JavaScript errors, requests to `/api`, external requests and outbound selected
bytes. Reports are retained under `dist/pages-check` and as GitHub Actions
artifacts. `deployment.json` identifies the actual source commit and explicitly
states `engine_hosted: false` and `C40_verified: false`.

## Deployment and rollback

Review the Pages workflow and its deployed URL in Settings → Pages. Verify the
live viewer, example download and `deployment.json` after deployment completes.
The `release` workflow and Environment are independent: the Pages request does
not authorize a signed v1 publication or its Environment approval. C40 stays
in progress until its separate published-artifact install proof succeeds.

For a website rollback, revert the isolated Pages source change and let the main
workflow deploy the reviewed version. This does not restore or overwrite local
user snapshots; the website stores none.

## Official upstream provenance

All three added Actions are MIT-licensed CI dependencies and are not bundled
application dependencies. Their release tags, exact commits and `action.yml`
were checked against the upstream repositories on 2026-10-20:

| Action | Release | Commit |
| --- | --- | --- |
| [actions/configure-pages](https://github.com/actions/configure-pages) | v6.0.0 | `45bfe0192ca1faeb007ade9deae92b16b8254a0d` |
| [actions/upload-pages-artifact](https://github.com/actions/upload-pages-artifact) | v5.0.0 | `fc324d3547104276b827a68afc52ff2a11cc49c9` |
| [actions/deploy-pages](https://github.com/actions/deploy-pages) | v5.0.1 | `368f82528645a54fb793d4d04e342629a3f51346` |

References: [GitHub custom Pages workflows](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages),
[Vite static deployment](https://vite.dev/guide/static-deploy).
