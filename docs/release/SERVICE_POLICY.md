# Supported profile

Attestara supports Linux x86_64, one local OS operator, immutable CLI snapshots
and a browser-local passport viewer. Other operating systems, multi-user access,
hosted sensitive workloads and remote domain APIs require separate implementation
and verification. OPA/Syft/Grype are optional integrations.

The core CLI uses bounded file/ZIP inputs up to 100 MiB and 2000 entries. These
limits are admission boundaries, not a capacity guarantee for arbitrary workloads.
The browser preview is separately limited to 1 MiB and keeps selected files only
in tab memory. No login/account service or automatic updater is provided.

Support and private vulnerability reporting follow [SECURITY.md](../../SECURITY.md).
There is no uptime, response-time or fix-deadline SLA.
