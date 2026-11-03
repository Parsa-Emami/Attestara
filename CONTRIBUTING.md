# Contributing

Read [the user guide](docs/release/USER_GUIDE.md) and the relevant source/tests
before changing behavior. Keep changes focused and use a Conventional Commit
that explains the resulting behavior and why it is needed.

Run `npm run setup`, `npm run check`, `go test ./...` and `go vet ./...`.
Browser changes also require `npm run test:browser`; packaging and installation
changes require `npm run test:package` and the clean-clone workflow documented in
[testing](docs/engineering/ENGINE_TESTS.md). Describe actual checks and their
limits in the PR; CI keeps generated reports outside the committed source.

Preserve Apache-2.0, upstream attribution, [DCO](DCO.md), input compatibility and
positive/negative regressions. Keep optional providers optional. Never disable
checks to hide failures or commit secrets, private inputs, dependency caches or
build output. Database schemas, migrations, .env files and Dockerfiles must not
be modified without explicit owner instructions for those files.

Ordinary bugs/support belong in [Issues](https://github.com/Parsa-Emami/Attestara/issues)
with the commit, platform/tool versions, reproduction and redacted expected/actual
result. Use synthetic inputs. Follow [SECURITY.md](SECURITY.md) for private reports.

## A focused first contribution

Start with an existing issue or open a [bounded improvement
request](https://github.com/Parsa-Emami/Attestara/issues/new?template=improvement.yml)
before a larger change. A good first patch can improve one concrete user-guide
scenario, its synthetic fixture and its regression assertion. Keep unsupported
capabilities and future release ideas clearly labelled.

When opening a pull request, identify the source revision, explain the behavior
before and after, and list the exact tests run. Check links and document the
supported platform when changing installation instructions. Automated checks
supplement, rather than replace, reproducible local verification.

For genuine joint work, identify each contributor's actual contribution in the
pull request. Use a `Co-authored-by: Name <verified-email>` commit trailer only
with the contributor's consent and their real GitHub-associated email. Never
assign co-authorship to people who did not contribute.
