# Open-source scope

Attestara source is available under [Apache-2.0](LICENSE). The supported local
core includes the CLI, browser viewer, rules, synthetic examples, tests, build
recipes and resource instructions. It requires no paid/cloud account,
proprietary entitlement or sibling repository. `package.json` has `private: true`
to prevent accidental npm registry publication.

Dependencies retain their own licenses. Versions, integrity metadata and retained
license texts are listed in [the resource lock](docs/engineering/toolchain-lock.json)
and [third-party notices](THIRD_PARTY_NOTICES.md). Optional OPA/Syft/Grype
integrations are not core prerequisites.

The [user guide](docs/release/USER_GUIDE.md) covers data, privacy, limits and
recovery. [Artifacts](docs/release/ARTIFACTS.md) explains distribution verification
and builds. Contributing and private security reports follow [CONTRIBUTING.md](CONTRIBUTING.md)
and [SECURITY.md](SECURITY.md).
