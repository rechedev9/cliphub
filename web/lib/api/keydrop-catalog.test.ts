import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { CSGOSKINS_STYLE_CATALOG, KEYDROP_STYLE_CATALOG } from './types.ts';

// A style offered in Studio must ship its preview plate: a missing file draws
// an empty banner in the editor (the Tigerr chip once did exactly that).
test('every affiliate style in the catalog ships its preview asset', () => {
  for (const entry of [...KEYDROP_STYLE_CATALOG, ...CSGOSKINS_STYLE_CATALOG]) {
    const file = join(process.cwd(), 'public', entry.preview);
    assert.ok(existsSync(file), `${entry.id}: missing ${entry.preview}`);
  }
});
