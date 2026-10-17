const fields = ['schemaVersion', 'status', 'scope', 'version', 'goVersion'];

export function validateEngineHealth(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length !== fields.length || fields.some(key => !Object.hasOwn(value, key)) || value.schemaVersion !== 1 || value.status !== 'ok' || value.scope !== 'engine-process' || value.version !== '0.0.1' || value.goVersion !== 'go1.27.2') throw new Error('Invalid engine process health.');
  return Object.freeze({ schemaVersion: 1, status: 'ok', scope: 'engine-process', version: String(value.version), goVersion: String(value.goVersion) });
}
