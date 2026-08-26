import { expect, test, type Page } from '@playwright/test';
import { SURFACE_RAMP, gotoStudio, parseOklch, rootToken } from './contract.ts';

const GENERIC_FONTS = ['inter', 'roboto', 'open sans', 'lato'];

async function fontFamilyOf(page: Page, selector: string): Promise<string> {
  return page.locator(selector).evaluate((node) => getComputedStyle(node).fontFamily);
}

/** Computed `background-color` is rgb/rgba/oklch; we only need the alpha. */
function computedAlpha(color: string): number {
  const trimmed = color.trim().toLowerCase();
  if (trimmed.startsWith('oklch')) return parseOklch(color).a;
  const slash = trimmed.match(/\/\s*([\d.]+%?)\s*\)/);
  if (slash?.[1] !== undefined) {
    return slash[1].endsWith('%') ? Number.parseFloat(slash[1]) / 100 : Number.parseFloat(slash[1]);
  }
  const rgba = trimmed.match(/rgba?\([^)]*\)/);
  if (rgba === null) return 1;
  const parts = rgba[0].replace(/rgba?\(|\)/g, '').split(/[\s,/]+/).filter(Boolean);
  if (parts.length < 4) return 1;
  const alpha = parts[3];
  return alpha.endsWith('%') ? Number.parseFloat(alpha) / 100 : Number.parseFloat(alpha);
}

test.describe('anti-slop chrome', () => {
  for (const href of ['/matches', '/upload'] as const) {
    test(`${href} body is Chakra Petch, not a generic AI typeface`, async ({ page }) => {
      await gotoStudio(page, href);
      const family = (await fontFamilyOf(page, 'body')).toLowerCase().replace(/_/g, ' ');
      expect(family).toMatch(/chakra\s*petch/);
      for (const banned of GENERIC_FONTS) {
        expect(family).not.toContain(banned);
      }
      const token = (await rootToken(page, '--font-sans')).toLowerCase().replace(/_/g, ' ');
      expect(token).toMatch(/chakra\s*petch/);
    });

    test(`${href} studio panels are opaque and unblurred`, async ({ page }) => {
      await gotoStudio(page, href);
      const panel = page.locator('.studio-panel').first();
      await expect(panel).toBeVisible();
      const chrome = await panel.evaluate((node) => {
        const style = getComputedStyle(node);
        return {
          backdropFilter: style.backdropFilter || style.getPropertyValue('backdrop-filter'),
          webkitBackdrop: style.getPropertyValue('-webkit-backdrop-filter'),
          backgroundImage: style.backgroundImage,
        };
      });
      expect(chrome.backdropFilter === 'none' || chrome.backdropFilter === '').toBe(true);
      expect(chrome.webkitBackdrop === 'none' || chrome.webkitBackdrop === '').toBe(true);
      expect(chrome.backgroundImage.toLowerCase()).not.toContain('backdrop');
    });
  }

  test('surface ramp steps are opaque', async ({ page }) => {
    await gotoStudio(page, '/matches');
    for (const { token } of SURFACE_RAMP) {
      const served = parseOklch(await rootToken(page, token));
      expect(served.a, token).toBe(1);
    }
  });

  test('keyboard focus on /matches is a visible cyan outline', async ({ page }) => {
    await gotoStudio(page, '/matches');
    let measured = false;
    for (let i = 0; i < 12 && !measured; i += 1) {
      await page.keyboard.press('Tab');
      const focus = await page.evaluate(() => {
        const node = document.activeElement;
        if (node === null || node === document.body) return null;
        const style = getComputedStyle(node);
        const box = node.getBoundingClientRect();
        return {
          outlineWidth: style.outlineWidth,
          outlineStyle: style.outlineStyle,
          outlineOffset: style.outlineOffset,
          outlineColor: style.outlineColor,
          visible: box.width > 0 && box.height > 0,
        };
      });
      if (focus === null || !focus.visible) continue;
      expect(focus.outlineStyle).not.toBe('none');
      expect(focus.outlineWidth).toBe('2px');
      expect(focus.outlineOffset).toBe('2px');
      const colour = parseOklch(focus.outlineColor);
      expect(colour.l).toBeCloseTo(0.811, 3);
      expect(colour.c).toBeCloseTo(0.135, 3);
      expect(colour.h).toBeCloseTo(207.6, 1);
      measured = true;
    }
    expect(measured).toBe(true);
  });

  test('the command strip is an opaque ceiling', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const strip = page.locator('[data-slot="shell-strip"]');
    await expect(strip).toBeVisible();
    const style = await strip.evaluate((node) => {
      const computed = getComputedStyle(node);
      return {
        backdropFilter: computed.backdropFilter,
        backgroundColor: computed.backgroundColor,
      };
    });
    expect(style.backdropFilter === 'none' || style.backdropFilter === '').toBe(true);
    expect(computedAlpha(style.backgroundColor)).toBe(1);
  });

  test('/matches empty state is the shared kit', async ({ page }) => {
    await gotoStudio(page, '/matches');
    await expect(page.locator('[data-slot="empty"]')).toBeVisible();
  });

  test('/upload dropzone keeps the real file input', async ({ page }) => {
    await gotoStudio(page, '/upload');
    await expect(page.locator('[data-slot="dropzone"]')).toBeVisible();
    const input = page.locator('input[type="file"]');
    await expect(input).toHaveCount(1);
    expect(await input.isEnabled()).toBe(true);
  });
});

const SCRATCH = process.env.GOAL_SCRATCH;

test.describe('studio screenshots', () => {
  test.skip(!SCRATCH, 'GOAL_SCRATCH is unset');

  test('matches and upload paint the navy room', async ({ page }) => {
    await gotoStudio(page, '/matches');
    expect(page.url()).toContain('/matches');
    await page.screenshot({ path: `${SCRATCH}/matches.png`, fullPage: true });
    await gotoStudio(page, '/upload');
    expect(page.url()).toContain('/upload');
    await page.screenshot({ path: `${SCRATCH}/upload.png`, fullPage: true });
  });
});
