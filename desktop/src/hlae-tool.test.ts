import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.4',
    archiveName: 'hlae_2_192_4.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.4/hlae_2_192_4.zip',
    sha256: '0718adfbb5e2786a85d454262a264892b94affba25ed07b8d9a97a5f57dcaae8',
    treeSha256: 'c03675d69354a8d923b7e41d6a60f6430d0229be74e20a5d3255b5061abadc7d',
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
