# Maintenance

Support targets the current Linux x86_64 single-operator CLI/snapshot profile and
browser-local passport viewer. [Published releases](https://github.com/Parsa-Emami/Attestara/releases)
retain the source and signatures for their own tagged commit. There is no automatic
updater, latest-release authority or multi-user service guarantee.

Node/npm/Go and upstream resources are pinned in
[toolchain-lock.json](../engineering/toolchain-lock.json). Dependency updates must
retain official provenance, compatible licenses, lockfiles and applicable notices.
Run the relevant engine/web/provider tests and the current-source release gate;
report actual validation in the PR and retain CI artifacts. Optional tools remain
optional. Do not treat a missing dependency or unknown license as a successful update.

Use [ARTIFACTS.md](ARTIFACTS.md) to verify an independently trusted signature,
checksums and expected source before installation. Upgrade by switching to a new
installation directory and reusing the external store. Rollback switches compatible
software; it never replaces newer data with an old backup. Restore into a new store.

Use Issues for ordinary support with synthetic reproductions. Private
vulnerabilities follow [SECURITY.md](../../SECURITY.md). Its response windows are
targets, not guaranteed deadlines or support SLAs.
