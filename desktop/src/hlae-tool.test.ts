import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.3',
    archiveName: 'hlae_2_192_3.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.3/hlae_2_192_3.zip',
    sha256: '680b90dd5bed3e5b17de0945cc62e147696817ecdcf5e80b5de3c3cb77a84ab1',
    treeSha256: 'acd15eb0580674d49c2233448d3734e6f6c74f1c55c9227094952ad4c6f4ddfe',
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
