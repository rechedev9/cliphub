import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import { PINNED_HLAE_TOOL } from './hlae-tool.ts';

test('pins the official HLAE release', () => {
  assert.deepEqual(PINNED_HLAE_TOOL, {
    version: '2.192.1',
    archiveName: 'hlae_2_192_1.zip',
    url: 'https://github.com/advancedfx/advancedfx/releases/download/v2.192.1/hlae_2_192_1.zip',
    sha256: '08ae68bb1c42c99bcd441f688d17e24bc52faed27eac07ebea5fc7c98e34b465',
    treeSha256: 'fc5bc770e8492d779fc9599838eab09e781be993de6683872578ddd0660cee54',
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
