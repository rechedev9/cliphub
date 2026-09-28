import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.6',
    archiveName: 'hlae_2_192_6.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.6/hlae_2_192_6.zip',
    sha256: 'b3acae70babb536e3b4a34fbbbe4ca8e55a1028068eaaf5fc98817775b72f4fa',
    treeSha256: '8aabba9993a775523802f9b7f42330e90bcd1bb294e84f510e192bbaa9f56e4a',
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
