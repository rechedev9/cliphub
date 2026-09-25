import test from 'node:test';
import assert from 'node:assert/strict';
import { hasMissingFullDemoFiles } from './full-demo-requirements.ts';

const VIDEO = { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) };
const slot = (value: boolean | 'file' = false) => ({ enabled: value !== false, video: value === 'file' ? VIDEO : null });

test('nothing enabled needs no file', () => {
  assert.equal(hasMissingFullDemoFiles({}), false);
  assert.equal(hasMissingFullDemoFiles({ bumpers: { intro: slot(), outro: slot() } }), false);
  assert.equal(hasMissingFullDemoFiles({ bumpers: { intro: slot(), outro: slot(), sponsor: slot() } }), false);
});

test('an enabled intro, sponsor or outro without its clip is missing a file', () => {
  assert.equal(hasMissingFullDemoFiles({ bumpers: { intro: slot(true), outro: slot() } }), true);
  assert.equal(hasMissingFullDemoFiles({ bumpers: { intro: slot(), outro: slot(true) } }), true);
  assert.equal(hasMissingFullDemoFiles({ bumpers: { intro: slot(), outro: slot(), sponsor: slot(true) } }), true);
  assert.equal(hasMissingFullDemoFiles({ bumpers: { intro: slot('file'), outro: slot('file'), sponsor: slot('file') } }), false);
});
