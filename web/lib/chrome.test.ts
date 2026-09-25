import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath } from 'node:url';

test('dropzone icon tile is an opaque surface, not alpha glass', () => {
  const src = readFileSync(
    fileURLToPath(new URL('../components/upload/demo-dropzone.tsx', import.meta.url)),
    'utf8',
  );
  assert.doesNotMatch(src, /backdrop-blur/);
  assert.doesNotMatch(src, /bg-surface-0\/\d+/);
  assert.match(src, /bg-surface-0 text-primary/);
});
