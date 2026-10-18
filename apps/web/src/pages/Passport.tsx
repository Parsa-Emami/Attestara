import { useEffect, useRef, useState } from 'react';
import { inspectJson } from '../lib/preview.mjs';

type EvidenceReference = { path: string; digest: string };
type PassportData = {
  schema_version: '1';
  status: 'complete' | 'partial' | 'unverified';
  release_id: string;
  source_revision: string;
  subject_digest: string;
  evidence_digest: string;
  policy_digest: string;
  policy_decision: 'pass' | 'fail' | 'unknown';
  verification: 'unverified' | 'rejected';
  evidence_references: EvidenceReference[];
  warnings: string[];
  report_digest: string;
};

const fields = new Set(['schema_version', 'status', 'release_id', 'source_revision', 'subject_digest', 'evidence_digest', 'policy_digest', 'policy_decision', 'verification', 'evidence_references', 'warnings', 'report_digest']);
const digest = /^sha256:[a-f0-9]{64}$/;
const path = /^[A-Za-z0-9_-][A-Za-z0-9._-]*(\/[A-Za-z0-9_-][A-Za-z0-9._-]*)*$/;
const record = (value: unknown): value is Record<string, unknown> => value !== null && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown): value is string => typeof value === 'string' && value.trim() !== '' && [...value].length <= 4096;

function readPassport(source: string, maxBytes: number): PassportData {
  const value = inspectJson(source, maxBytes).value as Record<string, unknown>;
  if (value.schema_version !== '1') throw new Error('Choose a version 1 Attestara passport.');
  if (Object.keys(value).length !== fields.size || Object.keys(value).some(key => !fields.has(key)) ||
      !text(value.release_id) || !text(value.source_revision) ||
      ![value.subject_digest, value.evidence_digest, value.policy_digest, value.report_digest].every(item => typeof item === 'string' && digest.test(item)) ||
      !['complete', 'partial', 'unverified'].includes(value.status as string) ||
      !['pass', 'fail', 'unknown'].includes(value.policy_decision as string) ||
      !['unverified', 'rejected'].includes(value.verification as string) ||
      !Array.isArray(value.evidence_references) || value.evidence_references.length > 2000 ||
      !value.evidence_references.every(item => record(item) && Object.keys(item).length === 2 && text(item.path) && path.test(item.path) && typeof item.digest === 'string' && digest.test(item.digest)) ||
      !Array.isArray(value.warnings) || !value.warnings.every(text)) {
    throw new Error('Choose a valid version 1 Attestara passport.');
  }
  const references = value.evidence_references as EvidenceReference[];
  if (value.status === 'complete' && value.policy_decision === 'unknown' ||
      value.status === 'unverified' && value.verification !== 'unverified' ||
      new Set(references.map(reference => reference.path)).size !== references.length ||
      references.some((reference, index) => index > 0 && reference.path < references[index - 1].path)) {
    throw new Error('Choose a valid version 1 Attestara passport.');
  }
  return value as PassportData;
}

export function Passport({ maxPreviewBytes }: { maxPreviewBytes: number }) {
  const [passport, setPassport] = useState<PassportData | null>(null);
  const [status, setStatus] = useState('No passport selected.');
  const [error, setError] = useState('');
  const [reading, setReading] = useState(false);
  const activeReader = useRef<FileReader | null>(null);
  const generation = useRef(0);

  useEffect(() => () => { generation.current++; activeReader.current?.abort(); }, []);

  function cancel() {
    generation.current++;
    activeReader.current?.abort();
    activeReader.current = null;
    setReading(false);
    setPassport(null);
    setError('');
    setStatus('File reading cancelled.');
  }

  function load(file: File | undefined) {
    if (!file) return;
    generation.current++;
    activeReader.current?.abort();
    activeReader.current = null;
    const current = generation.current;
    setPassport(null);
    setError('');
    setReading(false);
    if (file.size === 0) { setStatus('No passport selected.'); setError('Passport file is empty.'); return; }
    if (file.size > maxPreviewBytes) { setStatus('No passport selected.'); setError('File exceeds the preview limit.'); return; }
    const reader = new FileReader();
    activeReader.current = reader;
    setStatus('Reading local passport…');
    setReading(true);
    reader.onload = () => {
      if (current !== generation.current) return;
      activeReader.current = null;
      setReading(false);
      try {
        setPassport(readPassport(String(reader.result), maxPreviewBytes));
        setStatus('Passport loaded locally.');
      } catch (reason) {
        setStatus('No passport selected.');
        setError(reason instanceof Error ? reason.message : 'Unable to read this file.');
      }
    };
    const failed = () => {
      if (current !== generation.current) return;
      activeReader.current = null;
      setReading(false);
      setStatus('No passport selected.');
      setError('Unable to read this file.');
    };
    reader.onerror = failed;
    try { reader.readAsText(file); } catch { failed(); }
  }

  return <section className="panel" aria-label="Local passport viewer">
    <div className="eyebrow">Local passport viewer</div>
    <h2>Choose a release passport</h2>
    <p>Select a C19 passport JSON file. It stays in memory in this tab and is not uploaded or saved.</p>
    <label className="file-button">Choose passport file<input aria-label="Choose passport file" type="file" accept=".json,application/json" onChange={event => { load(event.target.files?.[0]); event.target.value = ''; }}/></label>
    <p className="muted">Maximum {Math.round(maxPreviewBytes / 1024)} KiB · local display only</p>
    <p role="status" aria-live="polite">{status}</p>
    {reading && <button type="button" onClick={cancel}>Cancel reading</button>}
    {error && <p role="alert" className="error">{error}</p>}
    {passport && <article className="preview">
      <h3>{passport.release_id}</h3>
      <p className="passport-assessment">Viewer assessment: Unverified</p>
      <p>Claimed policy decision: {passport.policy_decision}</p>
      <p className="muted">All fields below are claims from the selected file. This viewer has not checked the passport digest, referenced file bytes or signer trust.</p>
      <dl className="passport-details">
        <div><dt>Passport status</dt><dd>{passport.status}</dd></div>
        <div><dt>Passport verification field</dt><dd>{passport.verification}</dd></div>
        <div><dt>Source revision</dt><dd>{passport.source_revision}</dd></div>
        <div><dt>Subject digest</dt><dd>{passport.subject_digest}</dd></div>
        <div><dt>Evidence digest</dt><dd>{passport.evidence_digest}</dd></div>
        <div><dt>Policy digest</dt><dd>{passport.policy_digest}</dd></div>
        <div><dt>Passport digest</dt><dd>{passport.report_digest}</dd></div>
      </dl>
      <h3>Evidence references</h3>
      {passport.evidence_references.length ? <table className="passport-evidence"><thead><tr><th scope="col">Path</th><th scope="col">Claimed digest</th></tr></thead><tbody>{passport.evidence_references.map(reference => <tr key={reference.path}><td>{reference.path}</td><td>{reference.digest}</td></tr>)}</tbody></table> : <p>No evidence references.</p>}
      <h3>Warnings</h3>
      {passport.warnings.length ? <ul>{passport.warnings.map((warning, index) => <li key={index}>{warning}</li>)}</ul> : <p>No warnings.</p>}
      <button type="button" onClick={() => { setPassport(null); setStatus('No passport selected.'); }}>Clear passport</button>
    </article>}
  </section>;
}
