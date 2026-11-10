# Frequently asked questions

This FAQ covers the supported **local, single-operator** Attestara profile. For a reproducible walkthrough, start with the [user guide](USER_GUIDE.md).

## Is Attestara a hosted scanner or a multi-user service?

No. The supported profile is a local CLI, immutable evidence snapshots and a browser passport viewer for one Linux x86_64 operator. It does not provide hosted accounts, remote multi-user APIs or automatic vulnerability scanning. See the [installation guide](../engineering/CLONE_TO_RUN.md) and [threat model](../security/THREAT_MODEL.md).

## Does the browser viewer verify a passport?

No. The viewer displays claims in a selected local JSON file without uploading it. It does **not** verify referenced bytes, signatures or signer trust. Run the CLI verification steps in the [user guide](USER_GUIDE.md#evaluate-verify-and-view) before making a trust decision.

## Is a passing policy evaluation proof that a release is safe?

No. A policy decision, file integrity, signature validity and independently established signer trust are separate assessments. A passing `evidence_present` rule is not a malware scan, compliance certification or guarantee that a release is current. See the [CLI reference](../engineering/LOCAL_WORKFLOW.md).

## What platform and tools are supported?

Linux x86_64 with one local OS operator. Install Git, Node 24.19.0, npm 11.9.0 and Go 1.27.2, then run `npm run setup` and `npm start`. Other platforms are outside the currently documented support profile. The [installation guide](../engineering/CLONE_TO_RUN.md) includes the pinned source-build route.

## What if port 4173 is already in use?

Launch the app with `npm start -- --port 4301` and open `http://127.0.0.1:4301/#passport`. The app is intended for loopback access, not an internet-facing deployment. See the [web runbook](../engineering/WEB_RUNBOOK.md).

## Why does reusing an operation ID fail after changing an input?

Snapshots are immutable. An operation ID may replay when the stored input and result are identical; changed content or freshness under that ID conflicts. Use a **new** operation ID for a different snapshot, and do not overwrite an existing restore destination. See [preserve and recover snapshots](USER_GUIDE.md#preserve-and-recover-snapshots).

## How do I reproduce a working example safely?

After setup, run `bash scripts/reproduce-c30-guide.sh`. The script exercises isolated, licensed synthetic fixtures and checks observed success and tampered-byte exit codes. It does not prove security of unrelated releases. See the [C30 walkthrough](USER_GUIDE.md#reproduce-the-c30-cli-result-transcript).

## Where should I ask questions or report bugs?

Use the [issue forms](https://github.com/Parsa-Emami/Attestara/issues/new/choose) for ordinary support and reproducible bugs. Include the commit SHA, tool versions, exact command, redacted output and synthetic reproduction data. Follow [CONTRIBUTING.md](../../CONTRIBUTING.md) before submitting a fix.

**Do not disclose vulnerabilities in public issues, pull requests or discussions.** Use [private vulnerability reporting](../../SECURITY.md) instead.
