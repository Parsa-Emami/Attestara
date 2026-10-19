import test from 'node:test';
import assert from 'node:assert/strict';
import { checkSecurityReporting, reportURL } from '../../scripts/check-security-reporting.mjs';

test('private reporting check reads the setting without credentials or submitting a report', async () => {
  const result = await checkSecurityReporting(async (url, options) => {
    assert.equal(url, 'https://api.github.com/repos/Parsa-Emami/Attestara/private-vulnerability-reporting');
    assert.equal(options.method, 'GET');
    assert.equal(options.redirect, 'error');
    assert.ok(options.signal instanceof AbortSignal);
    assert.equal(options.headers.Authorization, undefined);
    assert.equal(options.body, undefined);
    return new Response('{"enabled":true}');
  });
  assert.equal(result.enabled, true);
  assert.equal(result.reportURL, reportURL);
});

test('disabled reporting stays disabled', async () => {
  assert.equal((await checkSecurityReporting(async () => new Response('{"enabled":false}'))).enabled, false);
});

test('unavailable, malformed and ambiguous responses cannot certify reporting', async () => {
  for (const status of [301, 403, 404, 429, 500]) {
    await assert.rejects(checkSecurityReporting(async () => new Response('{}', { status })), /unavailable/);
  }
  for (const body of ['{}', 'null', '{"enabled":"true"}', '{"enabled":1}', 'invalid']) {
    await assert.rejects(checkSecurityReporting(async () => new Response(body)));
  }
  await assert.rejects(checkSecurityReporting(async () => { throw new Error('offline'); }), /offline/);
});
