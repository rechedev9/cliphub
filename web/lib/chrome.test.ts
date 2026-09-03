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

test('shell strip class is opaque --surface-0, not a blur', () => {
  const css = readFileSync(fileURLToPath(new URL('../app/globals.css', import.meta.url)), 'utf8');
  const strip = css.match(/\.shell-strip\s*\{[^}]+\}/);
  assert.ok(strip, 'missing .shell-strip rule');
  assert.match(strip[0], /background-color:\s*var\(--surface-0\)/);
  assert.doesNotMatch(strip[0], /backdrop-filter/);
});

test('studio motion is CSS-only: no JS interpolator module', () => {
  const css = readFileSync(fileURLToPath(new URL('../app/globals.css', import.meta.url)), 'utf8');
  assert.match(css, /\.studio-shake\s*\{/);
  assert.doesNotMatch(css, /\.studio-ticker\s*\{/);
});
