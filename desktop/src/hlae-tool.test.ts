import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.7',
    archiveName: 'hlae_2_192_7.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.7/hlae_2_192_7.zip',
    sha256: 'c0e84832a14170718cb3bfa760144b42beb8dee62b95390e1602f61e1b990e62',
    treeSha256: '5153c917e48b273163f68737a39e27055a612da298f2bb8252e3867df7d17d8f',
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
