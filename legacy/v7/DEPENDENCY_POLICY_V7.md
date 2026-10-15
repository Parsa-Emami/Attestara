# Attestara — Dependency Policy v7

## Reference dependencies
- syft
- grype
- trivy optional adapter
- semgrep optional adapter
- opa
- cosign
- in-toto
- firecracker
- podman

All dependencies require exact source/license/security tracking. Vendor-specific libraries stay in Infrastructure adapters. Scanner/rule/model outputs are normalized before entering Domain. Third-party Actions/images are SHA/digest pinned in release paths.
