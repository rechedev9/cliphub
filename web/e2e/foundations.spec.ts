import { expect, test } from '@playwright/test';
import {
  BORDER_TOKENS,
  DURATION_TOKENS,
  FOREGROUND_RAMP,
  SIGNAL_TOKENS,
  SURFACE_RAMP,
  TYPE_SCALE,
  gotoStudio,
  parseMs,
  parseNumber,
  parseOklch,
  rootToken,
  squash,
} from './contract.ts';

/** Pin served CSS tokens so a ramp edit is a deliberate contract change. */
test.describe('foundations', () => {
  test.beforeEach(async ({ page }) => {
    await gotoStudio(page, '/matches');
  });

  for (const { token, value } of [...SURFACE_RAMP, ...FOREGROUND_RAMP, ...BORDER_TOKENS, ...SIGNAL_TOKENS]) {
    test(`${token} is served as declared`, async ({ page }) => {
      const served = parseOklch(await rootToken(page, token));
      expect(served.l).toBeCloseTo(value.l, 6);
      expect(served.c).toBeCloseTo(value.c, 6);
      expect(served.h).toBeCloseTo(value.h, 6);
      expect(served.a).toBeCloseTo(value.a, 6);
    });
  }

  for (const { token, ms } of DURATION_TOKENS) {
    test(`${token} is ${ms}ms`, async ({ page }) => {
      expect(parseMs(await rootToken(page, token))).toBe(ms);
    });
  }

  test('the surface ramp is monotonic in lightness and single-hue', async ({ page }) => {
    const served = await Promise.all(SURFACE_RAMP.map(({ token }) => rootToken(page, token)));
    const parsed = served.map((step) => parseOklch(step));

    for (let i = 1; i < parsed.length; i += 1) {
      expect(parsed[i].l, `surface-${i} must sit above surface-${i - 1}`).toBeGreaterThan(parsed[i - 1].l);
    }
    expect(new Set(parsed.map((step) => step.h)).size, 'the ramp is a single hue').toBe(1);
    // Alpha is never used to fake a layer. Compositing a panel
    // over the canvas at 94% is what made v3's panels measure 1.023:1.
    expect(parsed.every((step) => step.a === 1), 'a surface step carries alpha').toBe(true);
  });

  for (const { step, px, lineHeight, tracking } of TYPE_SCALE) {
    test(`${step} is ${px}px on the scale`, async ({ page }) => {
      expect(parseNumber(await rootToken(page, `--${step}`)) * 16).toBeCloseTo(px, 6);
      expect(parseNumber(await rootToken(page, `--${step}--line-height`))).toBeCloseTo(lineHeight, 6);
      if (tracking !== undefined) {
        expect(parseNumber(await rootToken(page, `--${step}--letter-spacing`))).toBeCloseTo(tracking, 6);
      }
    });
  }

  test('12px is the hard floor of the scale', async ({ page }) => {
    const sizes = await Promise.all(
      TYPE_SCALE.map(async ({ step }) => parseNumber(await rootToken(page, `--${step}`)) * 16),
    );
    expect(Math.min(...sizes)).toBeCloseTo(12, 6);
  });

  test('body copy renders at the 15/24 default', async ({ page }) => {
    const body = await page.evaluate(() => {
      const style = getComputedStyle(document.body);
      return { fontSize: style.fontSize, lineHeight: style.lineHeight };
    });
    expect(body.fontSize).toBe('15px');
    expect(body.lineHeight).toBe('24px');
  });

  test('elevation is bevel plus shadow, never a flat drop', async ({ page }) => {
    const edgeLight = parseOklch(await rootToken(page, '--edge-light'));
    const edgeShade = parseOklch(await rootToken(page, '--edge-shade'));
    expect(edgeLight.a).toBeCloseTo(0.075, 6);
    expect(edgeShade.a).toBeCloseTo(0.55, 6);

    // getPropertyValue substitutes nested var() references, so every level
    // arrives with the bevel already inlined.
    const bevel = squash(await rootToken(page, '--elev-bevel'));
    expect(bevel.startsWith('inset01px00')).toBe(true);
    expect(bevel).toContain('inset0-1px00');

    for (const level of [0, 1, 2, 3, 4, 5]) {
      const declared = squash(await rootToken(page, `--elev-${level}`));
      expect(declared, `--elev-${level} drops the bevel`).toContain(bevel);
      if (level > 0) {
        // Above the flat step every level must add a key and an ambient shadow.
        expect(declared.split('inset').length - 1, `--elev-${level} is bevel-only`).toBe(2);
        expect(declared.length).toBeGreaterThan(bevel.length);
      }
    }
  });

  test('the corner radius derives from one token', async ({ page }) => {
    expect(parseNumber(await rootToken(page, '--radius')) * 16).toBeCloseTo(10, 6);
  });
});
