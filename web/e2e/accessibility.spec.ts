import { expect, test } from '@playwright/test';
import { NAV_ROUTES, gotoStudio, parseOklch } from './contract.ts';

const INTERACTIVE = 'a[href], button, input, select, textarea, [role="button"], [role="tab"], [role="switch"]';

test.describe('focus', () => {
  test('keyboard focus paints a 2px outline with a 2px offset, not a ring', async ({ page }) => {
    await gotoStudio(page, '/clips');

    // Walk far enough into the shell to land on a real control rather than a
    // skip link, then measure whatever actually holds focus.
    let measured = false;
    for (let i = 0; i < 12 && !measured; i += 1) {
      await page.keyboard.press('Tab');
      const focus = await page.evaluate(() => {
        const node = document.activeElement;
        if (node === null || node === document.body) return null;
        const style = getComputedStyle(node);
        const box = node.getBoundingClientRect();
        return {
          tag: node.tagName.toLowerCase(),
          outlineWidth: style.outlineWidth,
          outlineStyle: style.outlineStyle,
          outlineOffset: style.outlineOffset,
          outlineColor: style.outlineColor,
          visible: box.width > 0 && box.height > 0,
        };
      });
      if (focus === null || !focus.visible) continue;

      expect(focus.outlineStyle, `${focus.tag} has no focus outline`).not.toBe('none');
      expect(focus.outlineWidth).toBe('2px');
      expect(focus.outlineOffset).toBe('2px');
      // A ring would paint a solid offset colour and halo every control inside
      // a panel; this must stay an outline.
      const colour = parseOklch(focus.outlineColor);
      expect(colour.l).toBeCloseTo(0.811, 3);
      expect(colour.c).toBeCloseTo(0.135, 3);
      expect(colour.h).toBeCloseTo(207.6, 1);
      measured = true;
    }
    expect(measured, 'no visible control took keyboard focus in 12 tabs').toBe(true);
  });
});

test.describe('target size', () => {
  for (const { name, href } of NAV_ROUTES) {
    test(`${name} keeps every interactive target at 40px or more`, async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 900 });
      await gotoStudio(page, href);

      // Poll: a fetch can paint an undersized icon for one frame.
      const measure = (): Promise<string[]> =>
        page.evaluate((selector) => {
        return [...document.querySelectorAll<HTMLElement>(selector)]
          .filter((node) => {
            const box = node.getBoundingClientRect();
            if (box.width === 0 || box.height === 0) return false;
            const style = getComputedStyle(node);
            if (style.visibility === 'hidden' || style.display === 'none') return false;
            // Hidden 1x1 file input; the visible label is the target.
            if (box.width <= 2 && box.height <= 2) return false;
            // WCAG 2.5.8: inline sentence links are exempt; button-styled ones are not.
            if (node.tagName === 'A' && style.display.startsWith('inline') && style.display !== 'inline-block') {
              return false;
            }
            return box.height < 40 || box.width < 40;
          })
          .slice(0, 8)
          .map((node) => {
            const box = node.getBoundingClientRect();
            const label = node.getAttribute('aria-label') ?? node.textContent?.trim().slice(0, 30) ?? '';
            return `${node.tagName.toLowerCase()} "${label}" ${Math.round(box.width)}x${Math.round(box.height)}`;
          });
        }, INTERACTIVE);

      await expect
        .poll(measure, { message: 'targets under 40px', timeout: 10_000 })
        .toEqual([]);
    });
  }
});

test.describe('names and live regions', () => {
  for (const { name, href } of NAV_ROUTES) {
    test(`${name} names every icon-only action`, async ({ page }) => {
      await gotoStudio(page, href);

      const measure = (): Promise<string[]> =>
        page.evaluate(() => {
          return [...document.querySelectorAll<HTMLElement>('button, a[href], [role="button"]')]
          .filter((node) => {
            const box = node.getBoundingClientRect();
            if (box.width === 0 || box.height === 0) return false;
            const text = node.textContent?.trim() ?? '';
            if (text.length > 0) return false;
            const named =
              node.getAttribute('aria-label') ??
              node.getAttribute('aria-labelledby') ??
              node.getAttribute('title');
              return named === null || named.trim() === '';
            })
            .slice(0, 8)
            .map((node) => `${node.tagName.toLowerCase()}.${node.className.slice(0, 60)}`);
        });

      await expect
        .poll(measure, { message: 'unnamed icon actions', timeout: 10_000 })
        .toEqual([]);
    });
  }
});

test.describe('preserved E2E hooks', () => {
  // The /clips/nueva file input is contract, not incidental markup.
  // The hooks that only exist once a roster arrives live in upload-roster.spec.
  test('the upload flow keeps its file input', async ({ page }) => {
    await gotoStudio(page, '/clips/nueva');
    const input = page.locator('input[type="file"]');
    await expect(input).toHaveCount(1);
    // The real input is visually hidden behind the dropzone, so the contract is
    // that it is present and operable, not that it is visible.
    expect(await input.isEnabled()).toBe(true);
  });
});
