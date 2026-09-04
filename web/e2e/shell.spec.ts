import { expect, test } from '@playwright/test';
import { MEASURE_SCALE, NAV_ROUTES, gotoStudio, parseNumber, rootToken, squash } from './contract.ts';

test.describe('shell geometry', () => {
  test('the sidebar is a 240px wall', async ({ page }) => {
    await gotoStudio(page, '/clips');
    const sidebar = page.locator('[data-slot="sidebar-container"]').first();
    await expect(sidebar).toBeVisible();
    const box = await sidebar.boundingBox();
    expect(box?.width).toBe(240);
  });

  test('sidebar rows are 48px', async ({ page }) => {
    await gotoStudio(page, '/clips');
    const rows = page.locator('[data-slot="sidebar-menu-button"]');
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(NAV_ROUTES.length);

    for (let i = 0; i < count; i += 1) {
      const row = rows.nth(i);
      if (!(await row.isVisible())) continue;
      const box = await row.boundingBox();
      expect(box?.height, `sidebar row ${i} is off the 48px grid`).toBe(48);
    }
  });

  test('the command strip is a 56px opaque band, never a backdrop blur', async ({ page }) => {
    await gotoStudio(page, '/clips');
    expect(parseNumber(await rootToken(page, '--shell-strip-height')) * 16).toBeCloseTo(56, 6);

    const strip = page.locator('header.shell-strip');
    await expect(strip).toBeVisible();
    expect((await strip.boundingBox())?.height).toBe(56);

    // A full-width sticky element that re-reads and two-pass blurs its backdrop
    // is the one effect the shell explicitly refuses (command-strip.tsx:26).
    const filter = await strip.evaluate((node) => getComputedStyle(node).backdropFilter);
    expect(filter === 'none' || filter === '').toBe(true);
  });

  test('the content column is capped at 1440px and pinned to the sidebar edge', async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 1080 });
    await gotoStudio(page, '/clips');
    const main = page.locator('main.\\@container\\/content').first();
    await expect(main).toBeVisible();
    const geometry = await main.evaluate((node) => {
      const style = getComputedStyle(node);
      return { maxWidth: style.maxWidth, marginLeft: style.marginLeft, marginRight: style.marginRight };
    });
    expect(geometry.maxWidth).toBe('1440px');
    // mr-auto, not mx-auto: the H1's left edge must not drift as the window grows.
    expect(geometry.marginLeft).toBe('0px');
    expect(geometry.marginRight).not.toBe('0px');
  });

  test('the content gutter is the fluid shell token', async ({ page }) => {
    await gotoStudio(page, '/clips');
    expect(squash(await rootToken(page, '--shell-gutter'))).toBe('clamp(1.5rem,3.2vw,4rem)');
  });

  for (const { token, className, px } of MEASURE_SCALE) {
    test(`${token} is ${px}px and .${className} serves it`, async ({ page }) => {
      await gotoStudio(page, '/clips');
      expect(parseNumber(await rootToken(page, token)) * 16).toBeCloseTo(px, 6);

      const served = await page.evaluate((name) => {
        const probe = document.createElement('div');
        probe.className = name;
        document.body.append(probe);
        const style = getComputedStyle(probe);
        const geometry = { maxWidth: style.maxWidth, width: style.width };
        probe.remove();
        return geometry;
      }, className);
      expect(served.maxWidth).toBe(`${px}px`);
    });
  }

  test('every shell route spines on the measure scale, never its own pixel width', async ({ page }) => {
    await page.setViewportSize({ width: 1920, height: 1080 });
    const allowed = new Set<number>(MEASURE_SCALE.map((step) => step.px));

    for (const { name, href } of NAV_ROUTES) {
      await gotoStudio(page, href);
      // Only the page's own spine — the direct children of <main>. A bounded
      // paragraph or a dialog deeper in the tree is line length, not measure.
      const spine = await page.evaluate(() => {
        const main = document.querySelector('main');
        if (main === null) return [];
        // The route entrance frame is chrome, not a page container; the page's
        // own spine is one level in when it is mounted.
        const root = main.querySelector('[data-slot="route-frame"]') ?? main;
        return [...root.children]
          .filter((node): node is HTMLElement => node instanceof HTMLElement)
          .map((node) => ({
            max: getComputedStyle(node).maxWidth,
            tag: node.tagName,
            cls: node.className.slice(0, 140),
          }));
      });

      for (const node of spine) {
        if (node.max === 'none') continue;
        expect(
          node.max.endsWith('px') && allowed.has(Math.round(Number.parseFloat(node.max))),
          `${name} (${href}) spines <${node.tag}> at max-width ${node.max}, off the measure scale: ${node.cls}`,
        ).toBe(true);
      }
    }
  });
});

test.describe('navigation state', () => {
  for (const { name, href } of NAV_ROUTES) {
    test(`${name} marks exactly one nav entry with aria-current`, async ({ page }) => {
      await gotoStudio(page, href);
      const current = page.locator('[data-slot="sidebar"] a[aria-current="page"]');
      await expect(current).toHaveCount(1);
      await expect(current).toHaveAttribute('href', href);
    });
  }

  test('the sidebar exposes every numbered section', async ({ page }) => {
    await gotoStudio(page, '/clips');
    for (const { href } of NAV_ROUTES) {
      await expect(page.locator(`[data-slot="sidebar"] a[href="${href}"]`).first()).toBeVisible();
    }
  });

  test('the root route lands on Clips y vídeos', async ({ page }) => {
    await gotoStudio(page, '/');
    await expect(page).toHaveURL(/\/clips$/);
  });
});

test.describe('app updates', () => {
  test('the command strip has no update control in the browser', async ({ page }) => {
    await gotoStudio(page, '/clips');
    await expect(page.getByTestId('app-update-control')).toHaveCount(0);
  });

  test('the command strip shows an update control when a release is available', async ({ page }) => {
    await page.addInitScript(installUpdateBridge);
    await gotoStudio(page, '/clips');
    const control = page.getByTestId('app-update-control');
    await expect(control).toBeVisible();
    await expect(control).toContainText(/Actualizar/i);
    const box = await control.boundingBox();
    expect(box?.height, 'update control is below the 40px target').toBeGreaterThanOrEqual(40);
  });

  test('the update control stays visible at 390px without horizontal overflow', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.addInitScript(installUpdateBridge);
    await gotoStudio(page, '/clips');
    await expect(page.getByTestId('app-update-control')).toBeVisible();
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow, 'command strip overflow at 390px').toBe(0);
  });
});

function installUpdateBridge(): void {
  const status = { state: 'available', version: '2.4.29', currentVersion: '2.4.28' };
  Object.defineProperty(window, 'cliphubUpdate', {
    value: {
      getStatus: async () => status,
      check: async () => ({ ok: true }),
      install: async () => ({ ok: true }),
      onStatus: () => () => undefined,
    },
  });
}
