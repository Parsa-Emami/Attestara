# Attestara: reference workflow

Status: planned domain acceptance; the current JSON structure preview does not implement this workflow.

Load valid sample SBOM with manifest -> evaluate local policy -> produce passport -> change one byte -> verify failure -> distinguish unsigned from signed/trusted.

Required invariants:

- No user source execution
- Digest binds subject and outputs
- Missing evidence never means pass

Required UAT:

- یک SBOM معتبر و policy را به passport با subject digest تبدیل کن
- subject عوض‌شده، manifest ناقص و امضای نامعتبر باید خطا یا unverified مشخص بدهند
- خروجی offline verifier و viewer دربارهٔ نتیجهٔ policy برابر باشند

Machine contract: tests/scenarios/clone-to-value.json. C01/C04 create and freeze the actual synthetic positive/negative fixture formats; no real secrets, user source/traffic/case data or unlicensed corpus. A clean source clone must reproduce the entire workflow with restart and restore at C30.
