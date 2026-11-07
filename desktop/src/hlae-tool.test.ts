import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.193.0',
    archiveName: 'hlae_2_193_0.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.193.0/hlae_2_193_0.zip',
    sha256: 'ac1f6740551cba7a72ba8437b84097d6f9a95d922ae05f333417bac6d1b746c2',
    treeSha256: '990bb7614a89ca7a0e93d914b7b680eb93c7469a0d559bf933ca818b732d1783',
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
