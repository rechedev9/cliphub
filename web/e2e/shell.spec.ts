import { expect, test } from '@playwright/test';
import { NAV_ROUTES, gotoStudio, parseNumber, rootToken, squash } from './contract.ts';

/** `/upload` is deliberately outside the authenticated group (design.md:137), so
 *  it has a compact standalone bar instead of the sidebar and command strip. */
const SHELL_ROUTES = NAV_ROUTES.filter((route) => route.href !== '/upload');

test.describe('shell geometry', () => {
  test('the sidebar is a 240px wall', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const sidebar = page.locator('[data-slot="sidebar-container"]').first();
    await expect(sidebar).toBeVisible();
    const box = await sidebar.boundingBox();
    expect(box?.width).toBe(240);
  });

  test('sidebar rows are 48px', async ({ page }) => {
    await gotoStudio(page, '/matches');
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
    await gotoStudio(page, '/matches');
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
    await gotoStudio(page, '/matches');
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
    await gotoStudio(page, '/matches');
    expect(squash(await rootToken(page, '--shell-gutter'))).toBe('clamp(1.5rem,3.2vw,4rem)');
  });
});

test.describe('navigation state', () => {
  for (const { name, href } of SHELL_ROUTES) {
    test(`${name} marks exactly one nav entry with aria-current`, async ({ page }) => {
      await gotoStudio(page, href);
      const current = page.locator('[data-slot="sidebar"] a[aria-current="page"]');
      await expect(current).toHaveCount(1);
      await expect(current).toHaveAttribute('href', href);
    });
  }

  test('the sidebar exposes every numbered section', async ({ page }) => {
    await gotoStudio(page, '/matches');
    for (const { href } of NAV_ROUTES) {
      await expect(page.locator(`[data-slot="sidebar"] a[href="${href}"]`).first()).toBeVisible();
    }
  });

  test('/upload runs outside the shell', async ({ page }) => {
    await gotoStudio(page, '/upload');
    await expect(page.locator('[data-slot="sidebar-container"]')).toHaveCount(0);
    await expect(page.locator('header.shell-strip')).toHaveCount(0);
  });

  test('the root route lands on Inicio', async ({ page }) => {
    await gotoStudio(page, '/');
    await expect(page).toHaveURL(/\/onboarding$/);
  });
});
