# Attestara acceptance gate

All scenarios are unexecuted in v0. Use synthetic or explicitly authorized owned data.

1. یک SBOM معتبر و policy را به passport با subject digest تبدیل کن
2. subject عوض‌شده، manifest ناقص و امضای نامعتبر باید خطا یا unverified مشخص بدهند
3. خروجی offline verifier و viewer دربارهٔ نتیجهٔ policy برابر باشند

Also require negative input/auth tests, crash or interrupted operation, clean install/upgrade, export/restore and workload bounds. Claims must match selected private/public distribution profile.
