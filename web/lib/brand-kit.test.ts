import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { brandBannerOf, brandKitProblem, brandSubscribeOf, brandWatermarkOf, exclusiveBanner, isBrandOverlay, parseBrandKit, type BrandKit } from './brand-kit.ts';
import { buildEditRequest } from './api/edit-request.ts';
import { parseEffectiveEditConfig } from './api/render-hydration.ts';
import { coerceEditConfig } from './api/reel-store.ts';
import { currentFullDemoOptions, isFullDemoOptions, isFullDemoSnapshot, withFullDemoStoryOverlay } from './full-demo-plan.ts';
import { FULL_DEMO_EDIT } from './full-demo.ts';
import type { EditConfig } from './api/types.ts';

const IMAGE = { id: '0b5d1b6e-8a3c-4f0e-9d8a-1c2b3d4e5f60', sha256: 'a'.repeat(64) };
const KIT: BrandKit = { logo: IMAGE, watermark_corner: 'bottom-left', watermark_opacity: 0.5, banner: IMAGE, banner_start_seconds: 2, banner_end_seconds: 6 };

function edit(overrides: Partial<EditConfig> = {}): EditConfig {
  return {
    format: 'short-9x16', killEffect: 'punch-in', transition: 'flash', intro: false, outro: false, hookText: false,
    killCounter: false, matchRecap: false, voiceComms: false, nativeHud: false, coverStrategy: 'generated-gameplay', ...overrides,
  };
}

test('the server kit reads without its dates and corners, and bad values are refused', () => {
  assert.deepEqual(parseBrandKit({ ...KIT, updated_at: '2026-11-15T10:00:00Z', corners: ['top-left'] }), KIT);
  assert.deepEqual(parseBrandKit({ watermark_corner: 'top-right', watermark_opacity: 0.8, banner_start_seconds: 0 }), { watermark_corner: 'top-right', watermark_opacity: 0.8, banner_start_seconds: 0 });
  assert.equal(parseBrandKit({ ...KIT, watermark_corner: 'middle' }), undefined);
  assert.equal(parseBrandKit({ ...KIT, logo: { id: 'x', sha256: IMAGE.sha256 } }), undefined);
  assert.equal(brandKitProblem({ ...KIT, watermark_opacity: 0.05 }), 'La opacidad debe estar entre el 10 % y el 100 %.');
  assert.equal(brandKitProblem({ ...KIT, banner_end_seconds: 2 }), 'El banner debe salir después de entrar.');
  assert.equal(brandKitProblem(KIT), null);
});

test('the kit copies are what the plan and the render keep', () => {
  assert.deepEqual(brandWatermarkOf(KIT), { image: IMAGE, corner: 'bottom-left', opacity: 0.5 });
  assert.deepEqual(brandBannerOf({ ...KIT, banner_end_seconds: undefined }), { image: IMAGE, start_seconds: 2 });
  assert.equal(brandWatermarkOf({ ...KIT, logo: undefined }), undefined);
  assert.equal(isBrandOverlay({ watermark: brandWatermarkOf(KIT), banner: brandBannerOf(KIT) }), true);
  assert.equal(isBrandOverlay({ watermark: { ...brandWatermarkOf(KIT), extra: 1 } }), false);
});

test('the brand banner and the affiliate banner turn each other off', () => {
  const affiliate = edit({ keyDropFamily: 'KEYDROP', keyDropStyle: 'classic' });
  const branded = exclusiveBanner(affiliate, { ...affiliate, brandBanner: true });
  assert.equal(branded.brandBanner, true);
  assert.equal(branded.keyDropStyle, '');
  const back = exclusiveBanner(branded, { ...branded, keyDropFamily: 'KEYDROP', keyDropStyle: 'classic' });
  assert.equal(back.brandBanner, false);
  assert.equal(back.keyDropStyle, 'classic');
});

test('brand switches travel for a Short and come back from the effective edit', () => {
  const body = buildEditRequest(edit({ brandWatermark: true, brandBanner: true }));
  assert.equal(body.brand_watermark, true);
  assert.equal(body.brand_banner, true);
  assert.equal('brand_banner' in buildEditRequest(edit({ brandBanner: true, keyDropFamily: 'KEYDROP', keyDropStyle: 'classic' })), false);
  assert.equal('brand_watermark' in buildEditRequest(edit({ brandWatermark: true, matchRecap: true })), false);
  assert.equal('brand_watermark' in buildEditRequest({ ...FULL_DEMO_EDIT, brandWatermark: true }), false);
  // The rendered edit holds the kit copy; the form only keeps that the switch was on.
  const rendered = parseEffectiveEditConfig({ ...body, brand_watermark: brandWatermarkOf(KIT), brand_banner: brandBannerOf(KIT) });
  assert.equal(rendered?.brandWatermark, true);
  assert.equal(rendered?.brandBanner, true);
  assert.equal(parseEffectiveEditConfig({ ...body, brand_watermark: { image: {} } })?.brandWatermark, undefined);
  assert.equal(coerceEditConfig({ ...edit(), brandWatermark: true, brandBanner: 'yes' }).brandBanner, undefined);
  assert.equal(coerceEditConfig({ ...edit(), brandWatermark: true }).brandWatermark, true);
});

test('a Full Demo plan keeps its brand copy and drops an empty one, as Go omits it', () => {
  const raw: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  const options = currentFullDemoOptions(raw.document.options);
  const brand = { watermark: brandWatermarkOf(KIT), banner: brandBannerOf(KIT) };
  const branded = { ...options, overlays: { ...options.overlays, brand } };
  assert.equal(isFullDemoOptions(branded), true);
  assert.deepEqual(currentFullDemoOptions(branded).overlays.brand, brand);
  assert.equal('brand' in currentFullDemoOptions({ ...options, overlays: { ...options.overlays, brand: {} } }).overlays, false);
  assert.equal(isFullDemoOptions({ ...options, overlays: { ...options.overlays, brand: { watermark: { ...brand.watermark, opacity: 0 } } } }), false);
});

test('the channel name is kept, checked and read back from the server', () => {
  assert.equal(parseBrandKit({ ...KIT, channel_name: 'ClipHub RSN' })?.channel_name, 'ClipHub RSN');
  assert.equal(parseBrandKit({ ...KIT, channel_name: ' ClipHub' }), undefined);
  assert.equal(brandKitProblem({ ...KIT, channel_name: 'x'.repeat(31) })?.includes('30'), true);
  assert.equal(brandKitProblem({ ...KIT, channel_name: 'ClipHub RSN' }), null);
  assert.deepEqual(brandSubscribeOf({ ...KIT, channel_name: 'ClipHub RSN' }), { logo: IMAGE, channel_name: 'ClipHub RSN' });
  assert.equal(brandSubscribeOf({ watermark_corner: 'top-right', watermark_opacity: 1, banner_start_seconds: 0 }), undefined);
});

test('the subscribe banner travels for a Short and comes back from the effective edit', () => {
  const body = buildEditRequest(edit({ subscribeBanner: true }));
  assert.equal(body.subscribe_banner, true);
  assert.equal('subscribe_banner' in buildEditRequest(edit()), false);
  assert.equal('subscribe_banner' in buildEditRequest(edit({ subscribeBanner: true, matchRecap: true })), false);
  assert.equal('subscribe_banner' in buildEditRequest({ ...FULL_DEMO_EDIT, subscribeBanner: true }), false);
  for (const copy of [{}, { channel_name: 'ClipHub RSN' }, { logo: IMAGE, channel_name: 'ClipHub RSN' }]) {
    assert.equal(parseEffectiveEditConfig({ ...body, subscribe_banner: copy })?.subscribeBanner, true);
  }
  assert.equal(parseEffectiveEditConfig({ ...body, subscribe_banner: { logo: {} } })?.subscribeBanner, undefined);
  assert.equal(coerceEditConfig({ ...edit(), subscribeBanner: true }).subscribeBanner, true);
});

test('the Full Demo subscribe button carries the kit card only while it is on', () => {
  const raw: unknown = JSON.parse(readFileSync(new URL('./full-demo-plan.fixture.json', import.meta.url), 'utf8'));
  assert.ok(isFullDemoSnapshot(raw));
  const options = currentFullDemoOptions(raw.document.options);
  const card = brandSubscribeOf({ ...KIT, channel_name: 'ClipHub RSN' });
  const on = withFullDemoStoryOverlay(options, 'subscribe', true, card);
  assert.deepEqual(on.overlays.brand?.subscribe, card);
  assert.equal(isFullDemoOptions(on), true);
  assert.deepEqual(currentFullDemoOptions(on).overlays.brand, { subscribe: card });
  assert.deepEqual(withFullDemoStoryOverlay(on, 'halftime', true).overlays.brand, { subscribe: card });
  const off = withFullDemoStoryOverlay(on, 'subscribe', false);
  assert.equal('brand' in off.overlays, false);
  assert.equal('brand' in currentFullDemoOptions({ ...off, overlays: { ...off.overlays, brand: { subscribe: card } } }).overlays, false);
  assert.equal('brand' in withFullDemoStoryOverlay(options, 'subscribe', true).overlays, false);
  assert.equal(isBrandOverlay({ subscribe: {} }), false);
});
