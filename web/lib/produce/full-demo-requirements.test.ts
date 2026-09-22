import test from 'node:test';
import assert from 'node:assert/strict';
import type { FullDemoOptions } from '../full-demo-plan.ts';
import { hasMissingFullDemoFiles } from './full-demo-requirements.ts';

const VIDEO = { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) };
type Required = Pick<FullDemoOptions, 'sponsor' | 'bumpers'>;

function options(patch: { sponsor?: Partial<FullDemoOptions['sponsor']>; intro?: boolean | 'file'; outro?: boolean | 'file'; bumpers?: false } = {}): Required {
  const slot = (value: boolean | 'file' | undefined) => ({ enabled: value !== undefined && value !== false, video: value === 'file' ? VIDEO : null });
  return {
    sponsor: { enabled: false, video: null, ...patch.sponsor } as FullDemoOptions['sponsor'],
    bumpers: patch.bumpers === false ? undefined : { intro: slot(patch.intro), outro: slot(patch.outro) },
  };
}

test('nothing enabled needs no file', () => {
  assert.equal(hasMissingFullDemoFiles(options()), false);
  assert.equal(hasMissingFullDemoFiles(options({ bumpers: false })), false);
});

test('an enabled sponsor without its video is missing a file', () => {
  assert.equal(hasMissingFullDemoFiles(options({ sponsor: { enabled: true } })), true);
  assert.equal(hasMissingFullDemoFiles(options({ sponsor: { enabled: true, video: VIDEO } })), false);
});

test('an enabled intro or outro without its clip is missing a file', () => {
  assert.equal(hasMissingFullDemoFiles(options({ intro: true })), true);
  assert.equal(hasMissingFullDemoFiles(options({ outro: true })), true);
  assert.equal(hasMissingFullDemoFiles(options({ intro: 'file', outro: 'file' })), false);
});

test('a sponsor that replaces its audio needs its narration', () => {
  const replace = { enabled: true, video: VIDEO, audio_policy: 'replace-narration' as const };
  assert.equal(hasMissingFullDemoFiles(options({ sponsor: { ...replace, narration: null } })), true);
  assert.equal(hasMissingFullDemoFiles(options({ sponsor: { ...replace, narration: VIDEO } })), false);
  assert.equal(hasMissingFullDemoFiles(options({ sponsor: { enabled: true, video: VIDEO, audio_policy: 'embedded', narration: null } })), false);
});
