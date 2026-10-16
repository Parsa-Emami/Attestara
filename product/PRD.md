# Attestara product requirements v0

Status: planning; application source not implemented. English-facing app/docs are the release default. See Persian assessment for commercial hypotheses.

Target user: maintainer متن‌باز و تیم کوچک DevSecOps

Job to complete: جمع‌کردن evidence و توضیح تصمیم release gate

Differentiation hypothesis: بررسی مستقل بستهٔ انتشار؛ جداکردن missing evidence از failed policy

## MVP acceptance scope

- CLI واردسازی SBOM و خروجی scanner موجود
- schema validation و subject binding
- policy توضیح‌پذیر و report قابل‌حمل
- PWA محلی برای مشاهدهٔ passport

## Excluded until another decision

- scanner server عمومی
- اجرای build/test کد کاربران
- Firecracker runners
- webhook GitHub App و چند سازمان

## End-to-end outcomes

1. یک SBOM معتبر و policy را به passport با subject digest تبدیل کن
2. subject عوض‌شده، manifest ناقص و امضای نامعتبر باید خطا یا unverified مشخص بدهند
3. خروجی offline verifier و viewer دربارهٔ نتیجهٔ policy برابر باشند

Functional requirements F01–F20 and nonfunctional/release requirements are mapped one-to-one in `plans/requirements.json`. Tests belong with their feature commit, not postponed until launch. Success: a clean installed selected profile performs the core task and recovers from documented failures. Public demand remains unvalidated until real trials are recorded.

## UI flow

Select/import scoped input -> validate limits and permission -> show actual processing state -> explain result and confidence -> review/triage where appropriate -> export -> recover in clean environment. Loading, empty, stale, partial, unauthorized, rejected input, timeout/cancel and permanent failure are explicit. Provide table/keyboard alternatives to canvas, charts and graph paths. No sensitive content in page telemetry.
