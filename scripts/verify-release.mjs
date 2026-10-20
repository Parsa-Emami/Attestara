import { createHash, createPublicKey, verify } from 'node:crypto';
import { constants } from 'node:fs';
import { open, lstat } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { parseArgs } from 'node:util';

const fail = (code, message) => { throw Object.assign(new Error(message), { code, exitCode: 2 }); };
const text = value => typeof value === 'string' && value.trim() && value.length <= 4096 && !/[\x00-\x1f\x7f]/.test(value);
const hex = (value, bytes) => typeof value === 'string' && new RegExp('^[a-f0-9]{' + bytes * 2 + '}$', 'i').test(value);
function shape(value, fields) {
  return value !== null && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === fields.length && fields.every(field => Object.hasOwn(value, field));
}
function json(bytes) {
  try { return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)); }
  catch { fail('invalid_json', 'Release metadata is not valid UTF-8 JSON.'); }
}
function checksumLines(bytes) {
  try { return new TextDecoder('utf-8', { fatal: true }).decode(bytes).split('\n'); }
  catch { fail('invalid_checksums', 'Release checksums must be valid UTF-8.'); }
}
async function readRegular(path, maximum, collect = false) {
  const file = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const info = await file.stat();
    if (!info.isFile() || info.size > maximum) fail('invalid_file', 'Release file must be regular and within its byte limit.');
    const digest = createHash('sha256'), buffer = Buffer.alloc(65536), chunks = [];
    let size = 0;
    for (;;) {
      const { bytesRead } = await file.read(buffer, 0, buffer.length, null);
      if (!bytesRead) break;
      size += bytesRead;
      if (size > maximum) fail('limit_exceeded', 'Release file exceeds its byte limit.');
      const bytes = buffer.subarray(0, bytesRead);
      digest.update(bytes);
      if (collect) chunks.push(Buffer.from(bytes));
    }
    if (size !== info.size) fail('changed_file', 'Release file changed while reading.');
    return { sha256: digest.digest('hex'), bytes: collect ? Buffer.concat(chunks) : undefined };
  } finally { await file.close(); }
}

// Trust and the expected source come from the operator, never from artifact metadata.
export async function verifyRelease({ directory, signature, trust, expectedSourceCommit, freshness } = {}) {
  const assessment = { schema_version: '1', scope: 'release-distribution', file_integrity: 'unverified', cryptographic_valid: false, signer_trusted: false, expected_source_match: false, freshness, release_trusted: false };
  try {
    if (!text(directory) || !text(trust) || (signature !== undefined && !text(signature)) || !hex(expectedSourceCommit, 20) || !['fresh', 'stale'].includes(freshness)) fail('invalid_arguments', 'Require a release directory, independent trust, full expected source commit and explicit fresh/stale classification.');
    if (!(await lstat(directory)).isDirectory()) fail('invalid_directory', 'Release root must be a real directory.');
    const configuration = json((await readRegular(trust, 65536, true)).bytes);
    if (!shape(configuration, ['schema_version', 'trusted_signers']) || configuration.schema_version !== '1' || !configuration.trusted_signers || typeof configuration.trusted_signers !== 'object' || Array.isArray(configuration.trusted_signers)) fail('invalid_trust', 'Independent trust configuration is invalid.');
    for (const [id, key] of Object.entries(configuration.trusted_signers)) if (!text(id) || !hex(key, 32)) fail('invalid_trust', 'Configured signer identity or key is invalid.');
    const sums = (await readRegular(join(directory, 'SHA256SUMS.txt'), 16384, true)).bytes;
    if (signature !== undefined) {
      const detached = json((await readRegular(signature, 16384, true)).bytes);
      if (!shape(detached, ['schema_version', 'signer_id', 'public_key_hex', 'signature_hex']) || detached.schema_version !== '1' || !text(detached.signer_id) || !hex(detached.public_key_hex, 32) || !hex(detached.signature_hex, 64)) fail('invalid_signature', 'Detached signature descriptor is invalid.');
      const key = createPublicKey({ key: { kty: 'OKP', crv: 'Ed25519', x: Buffer.from(detached.public_key_hex, 'hex').toString('base64url') }, format: 'jwk' });
      assessment.signer_trusted = Object.hasOwn(configuration.trusted_signers, detached.signer_id) && configuration.trusted_signers[detached.signer_id].toLowerCase() === detached.public_key_hex.toLowerCase();
      assessment.cryptographic_valid = verify(null, sums, key, Buffer.from(detached.signature_hex, 'hex'));
      if (!assessment.cryptographic_valid) fail('invalid_signature', 'Release checksum signature is invalid.');
    }
    const names = new Set(['release.json', 'sbom.cdx.json', 'attestara-' + expectedSourceCommit + '-linux-x64.tar.gz', 'attestara-' + expectedSourceCommit + '-source.tar.gz']);
    const lines = checksumLines(sums);
    if (lines.pop() !== '' || lines.length !== names.size) fail('invalid_checksums', 'Release checksums must list all four required artifacts exactly once.');
    let metadata;
    for (const line of lines) {
      const match = /^([a-f0-9]{64})  ([a-zA-Z0-9.-]+)$/.exec(line);
      if (!match || !names.delete(match[2])) fail('invalid_checksums', 'Release checksum path is missing, duplicate or unsupported.');
      const name = match[2];
      const file = await readRegular(join(directory, name), name === 'release.json' ? 65536 : 1024 * 1024 * 1024, name === 'release.json');
      if (file.sha256 !== match[1]) fail('checksum_mismatch', 'Release artifact checksum mismatch.');
      if (file.bytes) metadata = json(file.bytes);
    }
    assessment.file_integrity = 'verified';
    if (!metadata || metadata.schemaVersion !== 1 || metadata.sourceCommit !== expectedSourceCommit || !/^sha256:[a-f0-9]{64}$/.test(metadata.sourceIdentity) || metadata.platform !== 'linux-x64' || metadata.scope !== 'local-cli-snapshots') fail('release_mismatch', 'Release metadata differs from the expected source or supported profile.');
    assessment.expected_source_match = true;
    assessment.source_commit = metadata.sourceCommit;
    assessment.checksums_sha256 = createHash('sha256').update(sums).digest('hex');
    assessment.release_trusted = assessment.cryptographic_valid && assessment.signer_trusted && freshness === 'fresh';
    return { exitCode: assessment.release_trusted ? 0 : 1, assessment };
  } catch (error) {
    const inputFailure = ['ENOENT', 'ENOTDIR', 'ELOOP'].includes(error.code);
    assessment.error = { code: error.exitCode ? error.code : inputFailure ? 'invalid_file' : 'unavailable', message: error.exitCode ? error.message : inputFailure ? 'A required regular release file is missing or unsafe.' : 'Release verification could not complete.' };
    return { exitCode: error.exitCode ?? (inputFailure ? 2 : 3), assessment };
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  let result;
  try {
    const { values } = parseArgs({ options: Object.fromEntries(['directory', 'signature', 'trust', 'expected-source-commit', 'freshness'].map(name => [name, { type: 'string' }])) });
    result = await verifyRelease({ directory: values.directory, signature: values.signature, trust: values.trust, expectedSourceCommit: values['expected-source-commit'], freshness: values.freshness });
  } catch {
    result = { exitCode: 2, assessment: { schema_version: '1', scope: 'release-distribution', release_trusted: false, error: { code: 'invalid_arguments', message: 'Invalid release verification arguments.' } } };
  }
  process.stdout.on('error', () => { process.exitCode = 3; });
  process.stdout.write(JSON.stringify(result.assessment) + '\n', error => { process.exitCode = error ? 3 : result.exitCode; });
}
