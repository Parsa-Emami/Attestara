# C02 resource lock

`toolchain-lock.json` is a versioned inventory for the Linux x86_64 local profile. The existing web versions and npm lock are preserved. Go is the C03 build prerequisite; OPA, Syft and Grype are optional local tools. At C02, CycloneDX 1.7 and SPDX 3.0 were normative format references only. C10 and C11 later added read-only CycloneDX 1.7 and bounded SPDX 3.0.0 subset adapters; C12 added read-only Grype report projection, and C15 added fixed-policy OPA execution. Rego v1 is reported by the pinned OPA version; C02 required no external policy rule bundle, image or vulnerability database.

```sh
node scripts/verify-resources.mjs
node scripts/verify-resources.mjs --artifacts /absolute/path/to/downloads
node --test tests/web/resource-lock.test.mjs
```

The default command is offline: it checks official sources, immutable pins, retained license/checksum/commit snapshots, fixture hashes, the npm lock and required/optional declarations. It never downloads or executes content. The artifact command additionally hashes files named by each resource's `artifact.filename`; absent required archives fail, absent optional archives are reported, and an optional archive present with incorrect bytes fails. Exit 0 certifies resource integrity only. Actual tool version probes and source-build results are recorded separately in `docs/evidence/C02.json`.

## Obtaining and verifying resources

Download the exact `artifact.url` into an isolated directory using its recorded filename. Run the artifact command before extraction or execution. Node and the providers have independently published checksum files; npm has registry SHA-512 integrity. The retained upstream metadata is hashed too. Source archives without a detached published checksum are anchored to the official full Git commit: C02 reconstructed their Git trees and compared them to the trees in the official commit metadata, then recorded the archive SHA-256. This is distinct from claiming an upstream-published archive checksum.

On 2026-10-17 the normal Go 1.27.2 source/binary download endpoints returned HTTP 404, and the toolchain module proxy returned HTTP 403. The working route is the exact source archive in the lock, verified against official commit `022c8636110ebac86a9b88724cda168ed713c21f`, then built using the existing Go 1.26.2 bootstrap compiler:

```sh
cd /path/to/extracted-go/src
GOROOT_BOOTSTRAP=/path/to/go1.26.2 GOMAXPROCS=2 ./make.bash
../bin/go version
```

The official [Go source-build guide](https://go.dev/doc/install/source) describes bootstrap prerequisites. C02's actual compiler version and bootstrap executable hash are in its evidence. This build is of the upstream compiler, not the Attestara engine. The product `go.mod` and `go.sum` were added by C03 and are present in the current checkout.

Each provider's `source_build` records its immutable source archive, upstream recipe and command. Extract into a separate directory and use the verified Go compiler. Preserve the upstream `go.mod` and `go.sum`; build with `-mod=readonly`. If the host has an unavailable private Go proxy, a command-local `GOPROXY='https://proxy.golang.org|direct' GOSUMDB=sum.golang.org GOTOOLCHAIN=local` can use the public checksum-verified module route, with VCS fallback. It does not change project environment files. Optional provider source builds do not measure Attestara functionality or hardware capacity.

The observed Syft/Grype builds additionally used `GOPROXY='https://proxy.golang.org|https://goproxy.cn|direct'` because some public proxy and upstream download requests returned HTTP 403. The mirror was only a download transport: upstream `go.sum` and `GOSUMDB=sum.golang.org` remained active, and `go mod verify` passed for all three providers. It is not an application runtime service. Default source builds omit release metadata injection: Syft/Grype report `[not provided]` for their CLI version fields, while the source commits remain pinned. The default OPA source build also reports WebAssembly unavailable, unlike the downloaded release binary. No identical release-feature or bit-for-bit build claim is made.

Node's pinned [BUILDING.md](https://github.com/nodejs/node/blob/v24.19.0/BUILDING.md) preserves its source-build route; no new Node compiler build is claimed. npm's integrity-checked tarball is directly runnable JavaScript source. The validator uses the standard [Node Hash API](https://nodejs.org/docs/latest-v24.x/api/crypto.html#class-hash) and streaming file reads, with no added dependency. Complete distributed third-party packaging remains C31.

When intentionally updating the inventory, refresh the exact version/source/checksum/license snapshots and their hashes, the notices hash, and affected fixture hashes. Run all C02 checks and retain fresh source-bound evidence. Do not convert a network failure, missing license or unsupported required platform into a successful check.
