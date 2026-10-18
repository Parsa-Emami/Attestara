import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

export const suites = {
  './internal/licensecatalog': ['TestCatalogRejectsMissingAndModifiedBytes'],
  './cmd/attestara': ['TestHealthContractAndAdmission', 'TestRealListenerShutdownAndOccupiedPort', 'TestVersionAndInvalidLaunch'],
  './internal/contracts': ['TestC14ResultVersionsAndSchema', 'TestC04Fixtures', 'TestC04StrictJSON', 'TestC04LimitsAndCancellation', 'TestC04NestedFieldsResultsAndNoExecution'],
  './internal/storage': ['TestC14V2CrashRecovery', 'TestC14V2DiskFullAndLostACK', 'TestC14V2QuotaPreservesSnapshot', 'TestC05InjectedDiskFullRecovery', 'TestC05RealProcessCrashRecoveryAndWriterLock', 'TestC05CorruptionFailsClosedAndPreservesManifest', 'TestC05SourceBytesStayInertAndPrivate'],
  './tests': ['TestC15NoSourceExecutionAndEnvironment', 'TestC15RealOPAAndBindings', 'TestC15RejectsInvalidInputs', 'TestC15ProviderFailuresAndTimeout', 'TestC15FixtureProvenance', 'TestC15ReopenAndFreshProcess', 'TestC14FixtureProvenance', 'TestC14DecisionsAndExactBindings', 'TestC14RejectsInvalidRulesAndBytes', 'TestC14StoredV2AndV1Compatibility', 'TestC14EvidenceFactsStrictnessAndRuleByteIdentity', 'TestC05CommitRestartReplayAndConflict', 'TestC05SingleWriterAndCorruptReferences', 'TestC05RejectsInvalidBytesPathsAndInputBinding', 'TestC07EvidenceEnvelopeBindsSubjectToolConfigurationAndArtifact', 'TestC07EnvelopeRejectsInvalidFixtures', 'TestC07EnvelopeReloadPreservesBindings', 'TestC10ValidatesAndPreservesOriginalBytes', 'TestC10RejectsInvalidFixtures', 'TestC10LimitsStrictUTF8AndInertInput', 'TestC10NeverLoadsDocumentURLs', 'TestC10EmbeddedSchemaPins', 'TestC11NormalizedRoundTrip', 'TestC11RejectsInvalidFixtures', 'TestC11PurposesAndMergedDependencies', 'TestC11BoundsInertInputAndArchiveBoundary', 'TestC11InterruptedRead', 'TestC11NeverFetchesContextOrMetadata', 'TestC12GrypeMetadataFindingsAndNoSecretProjection', 'TestC12MissingCVEFeedRemainsUnknown', 'TestC12PreservesUnreportedVersionAndUnknownAge', 'TestC12RejectsMalformedAndOversizedReports', 'TestC12FreshProcessRepeat', 'TestC13RecognizedAndReviewExpressions', 'TestC13RejectsMalformedAndBoundedInput', 'TestC13SharedCatalogAndExceptionRoles', 'TestC13FreshProcessRepeat'],
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
