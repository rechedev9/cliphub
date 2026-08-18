import { expect, test } from '@playwright/test';
import { NAV_ROUTES, VALIDATION_WIDTHS, gotoStudio } from './contract.ts';

/** design.md:147 — measure horizontal overflow at every contract width. */
test.describe('horizontal overflow', () => {
  for (const width of VALIDATION_WIDTHS) {
    for (const { name, href } of NAV_ROUTES) {
      test(`${name} at ${width}px`, async ({ page }) => {
        await page.setViewportSize({ width, height: 900 });
        await gotoStudio(page, href);

        const overflow = await page.evaluate(() => {
          const root = document.documentElement;
          const offenders = [...document.querySelectorAll<HTMLElement>('body *')]
            .filter((node) => {
              const box = node.getBoundingClientRect();
              if (box.width === 0 || box.height === 0) return false;
              return box.right > root.clientWidth + 1;
            })
            .slice(0, 5)
            .map((node) => `${node.tagName.toLowerCase()}.${node.className.slice(0, 80)}`);
          return { scrollWidth: root.scrollWidth, clientWidth: root.clientWidth, offenders };
        });

        expect(
          overflow.scrollWidth,
          `overflows by ${overflow.scrollWidth - overflow.clientWidth}px; first offenders: ${overflow.offenders.join(' | ')}`,
        ).toBeLessThanOrEqual(overflow.clientWidth);
      });
    }
  }

  test('the shell survives 200% zoom', async ({ page }) => {
    // 200% zoom on a 1280px window is a 640px CSS viewport with everything at
    // double scale; emulating the resulting layout viewport is the measurable half.
    await page.setViewportSize({ width: 640, height: 450 });
    await gotoStudio(page, '/matches');
    const root = await page.evaluate(() => ({
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    expect(root.scrollWidth).toBeLessThanOrEqual(root.clientWidth);
  });
});

test.describe('container-keyed breakpoints', () => {
  test('the content column is a named container, not the viewport', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 900 });
    await gotoStudio(page, '/matches');

    const containerType = await page
      .locator('main.\\@container\\/content')
      .first()
      .evaluate((node) => getComputedStyle(node).containerType);
    expect(containerType).not.toBe('normal');

    // design.md:143 — content width is not the viewport; do not key on xl:.
    const contentBox = await page
      .locator('main.\\@container\\/content')
      .first()
      .evaluate((node) => {
        const style = getComputedStyle(node);
        return (
          node.getBoundingClientRect().width -
          Number.parseFloat(style.paddingLeft) -
          Number.parseFloat(style.paddingRight)
        );
      });
    expect(contentBox).toBeGreaterThan(940);
    expect(contentBox).toBeLessThan(980);
  });
});
