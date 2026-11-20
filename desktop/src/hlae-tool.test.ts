import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.193.1',
    archiveName: 'hlae_2_193_1.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.193.1/hlae_2_193_1.zip',
    sha256: '1d07333fa34aca23c9f994d77010591e60bf93e176debdf511a854fd6902c99e',
    treeSha256: '3cc4faaadb7c23a83c175b96f43db54835889960d609482cb478907aa2ccfd3a',
    kind: 'zip',
    exeRel: 'HLAE.exe',
    timeoutMs: 90_000,
  });
});

test('runtime and packaging use the same HLAE pin', () => {
  const manifest = JSON.parse(
    fs.readFileSync(new URL('./hlae-tool.json', import.meta.url), 'utf8'),
  );
  const { kind, ...runtimeManifest } = PINNED_HLAE_TOOL;

  assert.equal(kind, 'zip');
  assert.deepEqual(runtimeManifest, manifest);
});
