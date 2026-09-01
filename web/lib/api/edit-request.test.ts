import test from 'node:test';
import assert from 'node:assert/strict';
import { buildEditRequest } from './edit-request.ts';
import type { EditConfig } from './types.ts';

function edit(overrides: Partial<EditConfig> = {}): EditConfig {
  return {
    format: 'short-9x16',
    killEffect: 'punch-in',
    transition: 'flash',
    intro: false,
    outro: false,
    hookText: false,
    killCounter: false,
    matchRecap: false,
    voiceComms: false,
    nativeHud: false,
    coverStrategy: 'generated-gameplay',
    ...overrides,
  };
}

test('buildEditRequest persists family with the style so a later render cannot forget it', () => {
  const keyDrop = buildEditRequest(edit({ keyDropStyle: 'tigerr', keyDropCode: 'zackcsgo' }));
  assert.equal(keyDrop.keydrop_family, 'KEYDROP');
  assert.equal(keyDrop.keydrop_style, 'tigerr');
  assert.equal(keyDrop.keydrop_code, 'ZACKCSGO');

  const skins = buildEditRequest(
    edit({ keyDropFamily: 'CSGOSKINS', keyDropStyle: 'classic', keyDropCode: 'skins99' }),
  );
  assert.equal(skins.keydrop_family, 'CSGOSKINS');
  assert.equal(skins.keydrop_style, 'classic');
  assert.equal(skins.keydrop_code, 'SKINS99');

  const off = buildEditRequest(edit({ keyDropFamily: 'KEYDROP', keyDropStyle: '' }));
  assert.equal(off.keydrop_family, undefined);
  assert.equal(off.keydrop_style, undefined);
});
