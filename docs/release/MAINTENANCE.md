# Maintenance and dependency policy

## Supported maintenance scope

Maintenance targets the current `main` source on Linux x86_64 for the
single-operator `local-cli-snapshots` profile and browser-local passport viewer.
There is no stable v1 or historical release support line, and no automatic
updater or latest-release authority. Other platforms and remote or multi-user
deployments need their own implementation and evidence before they can be
described as supported. See the [service policy](SERVICE_POLICY.md) and
[security policy](../../SECURITY.md).

## Toolchains and dependencies

Required development/build resources are pinned in
[`toolchain-lock.json`](../engineering/toolchain-lock.json) and
[`RESOURCE_LOCK.md`](../engineering/RESOURCE_LOCK.md): Node 24.19.0, npm 11.9.0
and Go 1.27.2 for the verified Linux x86_64 source workflow. The npm lockfile
pins web dependencies. OPA, Syft and Grype are optional local tools; they are
not required to run the core workflow. Resource versions, source identities,
checksums and license texts are recorded in the resource inventory and
[`THIRD_PARTY_NOTICES.md`](../../THIRD_PARTY_NOTICES.md).

Dependency updates must use the official upstream source and a compatible
license. Update the lock, integrity metadata, license/notices inventory and
affected fixture hashes together. Run the resource checks and all relevant
tests on the supported profile, then record the tested source identity and
results in task evidence. Unknown license or integrity data, an unavailable
required resource, or an unsupported profile is a blocker; it must not be
silently treated as a successful update. Optional integrations remain
optional unless a separately reviewed change establishes otherwise.

## Changes, upgrades and vulnerability reports

Use the documented [artifact verification and upgrade procedure](ARTIFACTS.md)
for a reviewed local candidate. Verify the detached signature and expected
source identity before installing an artifact. Current upgrade evidence uses
synthetic test signing material; it does not establish an official maintainer
release identity or published release. The C40 [maintainer release procedure](RELEASING.md)
uses a separate pinned release key and requires public installed-workflow proof.
Rollback switches software while
preserving the external data store; it does not restore older data.

Report vulnerabilities through the private route and scope described in
[`SECURITY.md`](../../SECURITY.md). The published acknowledgement and
assessment windows are targets, not a fix deadline or service-level guarantee.
Do not send vulnerability details through public Issues. No response time,
uptime, fix deadline or compatibility promise beyond these documented targets
is guaranteed.

## Evidence and policy changes

Claims about platforms, capacity, providers, recovery or maintenance are limited
to current source-bound evidence. Update this policy when the supported
profile, resource locks, security reporting route or tested recovery behavior
changes. Release readiness and publication remain separate gates documented by
C39 and C40.
