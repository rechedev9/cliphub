import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.193.2',
    archiveName: 'hlae_2_193_2.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.193.2/hlae_2_193_2.zip',
    sha256: '44eec643c4abbe2965b17a0b5e0a9afce5cb77f437b1907a5dd359c021e0dd86',
    treeSha256: '95611999d06febf85c0a946c5bee484a6691e9db3908db12929f89d1c8fd0fbc',
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
