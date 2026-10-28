import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.10',
    archiveName: 'hlae_2_192_10.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.10/hlae_2_192_10.zip',
    sha256: 'cac5bb990f59110c31b159b5d008aa85e6332f04bbaf6b87e82b3228606bc28d',
    treeSha256: '3d5c0437511241eacf79b2baf359e9909b55bb3ceae8df9078cb362153ee8ad7',
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
