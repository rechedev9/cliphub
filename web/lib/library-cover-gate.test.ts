import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';

test('the publish screen never requires a cover pick before MP4', () => {
  const src = readFileSync(
    join(dirname(fileURLToPath(import.meta.url)), '../app/(app)/clips/[id]/publicar/[clipId]/page.tsx'),
    'utf8',
  );
  assert.equal(src.includes('selectVideoCover'), false);
  assert.equal(src.includes('elige candidata'), false);
  assert.equal(src.includes('coverApproved'), false);
  assert.match(src, /disabled=\{!video\.downloadUrl\}/);
});
