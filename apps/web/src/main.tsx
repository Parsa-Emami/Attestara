import { StrictMode, useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import project from './generated/project.json';
import { validateRuntimeConfig } from './lib/runtime-config.mjs';
import { inspectJson } from './lib/preview.mjs';
import { validateEngineHealth } from './lib/engine-health.mjs';
import { Passport } from './pages/Passport';
import './styles.css';

type Runtime = { projectId: string; apiBasePath: string; engineEnabled: false; maxPreviewBytes: number };
type Route = 'workspace' | 'passport';
const staticPages = __ATTESTARA_STATIC_PAGES__;
const staticAvailability = 'Online viewer. Files stay in this tab. The Go engine and snapshot operations run in the local application.';
function route(): Route { return location.hash === '#passport' ? 'passport' : 'workspace'; }

function App({ config }: { config: Runtime }) {
  const [page, setPage] = useState<Route>(route);
  const [preview, setPreview] = useState<{ name: string; bytes: number; fields: string[] } | null>(null);
  const [error, setError] = useState('');
  const busy = useRef(false);
  const generation = useRef(0);
  const [engine, setEngine] = useState<ReturnType<typeof validateEngineHealth> | null>(null);
  const [engineState, setEngineState] = useState(staticPages ? staticAvailability : 'Checking local engine…');
  useEffect(() => {
    if (staticPages) return;
    let active = true;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 3000);
    fetch('/api/engine/health', { cache: 'no-store', credentials: 'omit', signal: controller.signal }).then(async response => {
      if (!response.ok) throw new Error('Engine unavailable');
      return validateEngineHealth(await response.json());
    }).then(health => { if (!controller.signal.aborted) { setEngine(health); setEngineState('Local engine process was healthy at the last check.'); } }).catch(() => {
      if (active) { setEngine(null); setEngineState('Local engine was unavailable at the last check. Run setup/start and reload to retry.'); }
    }).finally(() => clearTimeout(timeout));
    return () => { active = false; clearTimeout(timeout); controller.abort(); };
  }, []);
  useEffect(() => {
    const onRoute = () => { setPage(route()); document.getElementById('content')?.focus(); };
    addEventListener('hashchange', onRoute);
    return () => removeEventListener('hashchange', onRoute);
  }, []);
  async function importFile(file: File | undefined) {
    if (!file || busy.current) return;
    const current = ++generation.current;
    setError(''); setPreview(null);
    if (file.size > config.maxPreviewBytes) { setError('File exceeds the preview limit.'); return; }
    busy.current = true;
    try {
      const result = inspectJson(await file.text(), config.maxPreviewBytes);
      if (current === generation.current) setPreview({ name: file.name, fields: result.fields, bytes: result.bytes });
    } catch (reason) {
      if (current === generation.current) setError(reason instanceof Error ? reason.message : 'Unable to read this file.');
    } finally { busy.current = false; }
  }
  return <div className="app" style={{ '--accent': project.accent } as React.CSSProperties}>
    <a className="skip" href="#content" onClick={event => { event.preventDefault(); document.getElementById('content')?.focus(); }}>Skip to content</a>
    <header><a className="brand" href="#workspace"><span className="mark" aria-hidden="true">{project.name.slice(0, 1)}</span>{project.name}</a><nav aria-label="Main"><a href="#workspace" aria-current={page === 'workspace' ? 'page' : undefined}>Workspace</a><a href="#passport" aria-current={page === 'passport' ? 'page' : undefined}>Passport</a></nav><span className="badge">{staticPages ? 'Online viewer' : 'Local application'}</span></header>
    <main id="content" tabIndex={-1}>
      <div className="eyebrow">{project.mode.replaceAll('-', ' ')} · browser workspace</div>
      <h1>{page === 'workspace' ? project.title : 'Inspect a release passport.'}</h1>
      <p className="intro">{page === 'workspace' ? project.description : 'Read a local passport and its evidence claims without uploading the file.'}</p>
      {staticPages && <section className="panel" aria-label="Online and local use"><div className="eyebrow">Start here</div><h2>Open your passport. Keep your files local.</h2><p>Use Passport to inspect an existing release passport, or Workspace to preview JSON. Selected files stay in this tab. Their contents are not sent to GitHub.</p><p>For policy evaluation, trusted signature assessment, durable snapshots, backup, restore and rollback, run the full local application on Linux. GitHub Pages hosts this viewer; it does not run the Go engine.</p><a className="text-link" href="https://github.com/Parsa-Emami/Attestara/blob/main/docs/engineering/CLONE_TO_RUN.md" target="_blank" rel="noreferrer">Set up the full local application →</a><p><a className="text-link" href={`${import.meta.env.BASE_URL}example-passport.json`} download>Download an example passport</a><span className="muted"> · Example data for learning; all passport fields remain unverified claims.</span></p></section>}
      {page === 'workspace' ? <>
        <section className="notice" aria-label="Availability" aria-live="polite"><span className="dot"/><span>{engineState}{engine && ` Engine ${engine.version} · ${engine.goVersion}.`} The browser displays files; use the CLI for evaluation and verification.</span></section>
        <div className="grid"><section className="panel"><div className="eyebrow">Local file preview</div><h2>Inspect a document</h2><p>Choose a JSON file to inspect its top-level fields. The file stays in memory in this tab and is not uploaded or saved.</p><label className="file-button">Choose JSON file<input aria-label="Choose JSON file" type="file" accept=".json,application/json" onChange={event => { void importFile(event.target.files?.[0]); event.target.value = ''; }}/></label><p className="muted">Maximum {Math.round(config.maxPreviewBytes / 1024)} KiB · JSON structure only</p>
          {error && <p role="alert" className="error">{error}</p>}
          {preview && <div className="preview" role="status"><h3>{preview.name}</h3><p>{preview.bytes} bytes · {preview.fields.length} top-level fields</p><ul>{preview.fields.slice(0, 30).map(field => <li key={field}><code>{field}</code></li>)}</ul><p className="muted">JSON structure checked. Domain validation requires the engine.</p><button onClick={() => { generation.current++; setPreview(null); }}>Clear preview</button></div>}
        </section><section className="panel"><div className="eyebrow">Product contract</div><h2>{project.documentType.replaceAll('-', ' ')}</h2><p>The local CLI keeps policy decisions, file integrity and signer trust separate.</p><ul className="invariants">{project.invariants.map(invariant => <li key={invariant}>{invariant}</li>)}</ul><a className="text-link" href="https://github.com/Parsa-Emami/Attestara/blob/main/docs/release/USER_GUIDE.md" target="_blank" rel="noreferrer">Read the user guide →</a></section></div>
      </> : <>
        <section className="notice" aria-label="Availability" aria-live="polite"><span className="dot"/><span>{engineState} Local passport viewing does not require the engine.</span></section>
        <Passport maxPreviewBytes={config.maxPreviewBytes}/>
      </>}
    </main><footer><span>{project.name}</span><span>No telemetry · No automatic file uploads</span></footer>
  </div>;
}

const root = createRoot(document.getElementById('root')!);
fetch(`${import.meta.env.BASE_URL}runtime-config.json`, { cache: 'no-store', credentials: 'omit' }).then(async response => {
  if (!response.ok) throw new Error('Workspace configuration is unavailable.');
  return validateRuntimeConfig(await response.json(), project.slug) as Runtime;
}).then(config => root.render(<StrictMode><App config={config}/></StrictMode>)).catch(() => {
  root.render(<main className="startup-error"><h1>Workspace unavailable</h1><p>The workspace configuration could not be loaded. Reload the page or contact the operator.</p></main>);
});
