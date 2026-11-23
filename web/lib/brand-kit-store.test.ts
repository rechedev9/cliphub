import test from 'node:test';
import assert from 'node:assert/strict';
import { DEFAULT_BRAND_KIT, type BrandKit } from './brand-kit.ts';
import { createBrandKitStore } from './brand-kit-store.ts';
import { serverError } from './human-error.ts';

const KIT: BrandKit = { ...DEFAULT_BRAND_KIT };

test('two subscribers share one read, and a save reaches both', async () => {
  let reads = 0;
  const store = createBrandKitStore(async () => { reads += 1; return KIT; });
  let seenA = 0;
  let seenB = 0;
  const offA = store.subscribe(() => { seenA += 1; });
  const offB = store.subscribe(() => { seenB += 1; });
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(reads, 1);
  assert.equal(store.snapshot().kit, KIT);
  const saved = { ...KIT, watermark_opacity: 0.5 };
  store.setKit(saved);
  assert.equal(store.snapshot().kit, saved);
  assert.deepEqual([seenA, seenB], [2, 2]);
  offA();
  offB();
});

test('the last unsubscribe drops a read still in flight, and the next subscriber reads again', async () => {
  let reads = 0;
  let release: (kit: BrandKit) => void = () => {};
  const store = createBrandKitStore(() => { reads += 1; return new Promise((resolve) => { release = resolve; }); });
  store.subscribe(() => {})();
  release(KIT);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(store.snapshot().kit, null);
  store.subscribe(() => {});
  assert.equal(reads, 2);
});

test('a failed read keeps the kit already shown and reports why', async () => {
  const store = createBrandKitStore(async () => { throw new Error('sin conexión'); });
  store.setKit(KIT);
  store.subscribe(() => {});
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(store.snapshot(), { kit: KIT, error: 'sin conexión' });
});

test('a failed server read reports a sentence of ours, not the server text', async () => {
  const store = createBrandKitStore(async () => { throw serverError('brand kit store: disk full', { status: 500 }); });
  store.subscribe(() => {});
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(store.snapshot().error, 'No se pudo leer el kit de marca. Vuelve a intentarlo.');
});
