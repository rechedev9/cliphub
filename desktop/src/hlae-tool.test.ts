import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.2-cliphub.1',
    archiveName: 'hlae_2_192_2_cliphub_1.zip',
    url: 'https://github.com/rechedev9/advancedfx/releases/download/v2.192.2-cliphub.1/hlae_2_192_2_cliphub_1.zip',
    sha256: 'b27d92058ee988d4da5cae60148fe437378d3229eec70f64e7ecdf0b0c2ddf19',
    treeSha256: 'a69ae0286da696f1925bee3497f14b89b9677c5ed2686df91a358c3f21614f82',
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
