import { createReadStream } from 'node:fs';
import { createInterface } from 'node:readline';
import { pathToFileURL } from 'node:url';

const prefix = 'github.com/rechedev9/cliphub/internal/';
export const requiredTests = [
  ['recording', 'TestGeneratedHLAEScriptRunsInMIRVSimulator'],
  ['editor', 'TestShortsParsePlanPortraitSeam'],
  ['editor', 'TestFullDemoOverlayCompositesOntoFixtureCapture'],
  ['editor', 'TestFullDemoConcatsTwoFixtureRounds'],
  ['recording', 'TestFullDemoExactRuntimeInExistingMIRVSimulator'],
  ['editor', 'TestFullDemoMasterDecodedAAC'],
  ['editor', 'TestFullDemoDecodedDuckingAndExplicitZero'],
  ['voicecomms', 'TestDecodedTeamVoiceAfterLongSilenceAndSideChange'],
  ['editor', 'TestFullDemoSponsorAndPlaylistMediaCanary'],
].map(([pkg, test]) => `${prefix}${pkg}/${test}`);

// A skipped, renamed or removed canary must not silently turn this lane green.
export async function verifyEvidence(lines) {
  const passed = new Set();
  for await (const line of lines) {
    if (!line.trim()) continue;
    const event = JSON.parse(line);
    const key = `${event.Package}/${event.Test}`;
    if (event.Action === 'fail') throw new Error(`Go test failed: ${key}`);
    if (event.Action === 'skip' && requiredTests.some(test => key === test || key.startsWith(`${test}/`))) {
      throw new Error(`Critical flow skipped: ${key}`);
    }
    if (event.Action === 'pass') passed.add(key);
  }
  const missing = requiredTests.filter(test => !passed.has(test));
  if (missing.length) throw new Error(`Missing critical flow passes:\n${missing.join('\n')}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (!process.argv[2]) throw new Error('Usage: node scripts/ci-backend-evidence.mjs <go-test.jsonl>');
  await verifyEvidence(createInterface({ input: createReadStream(process.argv[2]), crlfDelay: Infinity }));
  console.log('Critical capture, Shorts and Full Demo tests passed without skips.');
}
