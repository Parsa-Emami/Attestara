import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

export const suites = {
  './cmd/attestara': ['TestHealthContractAndAdmission', 'TestRealListenerShutdownAndOccupiedPort', 'TestVersionAndInvalidLaunch'],
  './internal/contracts': ['TestC04Fixtures', 'TestC04StrictJSON', 'TestC04LimitsAndCancellation', 'TestC04NestedFieldsResultsAndNoExecution'],
  './internal/storage': ['TestC05InjectedDiskFullRecovery', 'TestC05RealProcessCrashRecoveryAndWriterLock', 'TestC05CorruptionFailsClosedAndPreservesManifest', 'TestC05SourceBytesStayInertAndPrivate'],
  './tests': ['TestC05CommitRestartReplayAndConflict', 'TestC05SingleWriterAndCorruptReferences', 'TestC05RejectsInvalidBytesPathsAndInputBinding'],
};

export function validateDiscovery(pkg, output) {
  const found = new Set(output.split(/\r?\n/));
  const missing = suites[pkg].filter(name => !found.has(name));
  if (missing.length) throw new Error(`${pkg}: required engine tests missing: ${missing.join(', ')}`);
}

const root = fileURLToPath(new URL('../', import.meta.url));
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    for (const pkg of Object.keys(suites)) {
      const run = spawnSync('go', ['test', '-list', '^Test', pkg], {
        cwd: root, encoding: 'utf8', timeout: 120000, maxBuffer: 1024 * 1024,
      });
      if (run.error || run.status !== 0) throw new Error(`${pkg}: discovery failed: ${run.error?.message ?? run.stderr ?? run.signal}`);
      validateDiscovery(pkg, run.stdout);
      console.log(`${pkg}: required tests discovered`);
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
