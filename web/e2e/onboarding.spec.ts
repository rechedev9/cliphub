import { expect, test } from '@playwright/test';
import { VALIDATION_WIDTHS, gotoStudio } from './contract.ts';

/** The three ways in, in the order the screen offers them. */
const DOORS = [
  { href: '/upload', title: 'Sube una demo' },
  { href: '/streams', title: 'Corta un stream' },
  { href: '/players', title: 'Busca un jugador' },
] as const;

test.describe('Inicio', () => {
  test('is the first key in the rail, numbered 00', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const keys = page.locator('[data-slot="sidebar"] a[href^="/"]');
    // evaluateAll does not auto-wait, so anchor on a visible key first rather
    // than reading an empty list off a page that has not painted.
    await keys.first().waitFor({ state: 'visible' });
    const hrefs = await keys.evaluateAll((nodes) => nodes.map((node) => node.getAttribute('href')));
    // The wordmark links to /matches, so the first *nav* key is the one after it.
    const navHrefs = hrefs.filter((href) => href !== null);
    expect(navHrefs.indexOf('/onboarding')).toBeLessThan(navHrefs.indexOf('/matches', 1));

    const key = page.locator('[data-slot="sidebar"] a[href="/onboarding"]');
    await expect(key).toBeVisible();
    await expect(key).toContainText('00');
    await expect(key).toContainText('Inicio');
  });

  test('sits above the phase groups without stealing their heading', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    // textContent, not innerText: the labels are uppercased in CSS, and reading
    // the rendered casing would assert the stylesheet instead of the nav model.
    const labels = await page.locator('[data-slot="sidebar-group-label"]').allTextContents();
    // Three phases, and no fourth heading invented for the single entry key.
    expect(labels.map((label) => label.trim())).toEqual(['Producción', 'Salida']);
  });

  test('offers exactly three doors, each a whole-card link', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    for (const { href, title } of DOORS) {
      const door = page.locator(`main a[href="${href}"]`);
      await expect(door).toHaveCount(1);
      await expect(door).toContainText(title);
      const box = await door.boundingBox();
      expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
    }
    expect(await page.locator('main a[href^="/"]').count()).toBe(DOORS.length);
  });

  test('lights exactly one door', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    const doors = page.locator('main a[href^="/"]');
    await expect(doors).toHaveCount(DOORS.length);
    const glowing = await doors.evaluateAll((nodes) =>
      nodes.filter((node) => getComputedStyle(node).boxShadow.includes('0px 0px 26px')).length,
    );
    // Brand emphasis is one restrained moment, not a row of them.
    expect(glowing).toBe(1);
  });

  test('explains the four stages without inventing progress', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    // Scoped to the plate: Inicio carries a second ordered list (the share-code
    // steps), and an unscoped `main ol li` counts both.
    const stages = page.locator('main [data-slot="guide-plate"] ol li');
    await expect(stages).toHaveCount(4);
    for (const [index, label] of ['COLA', 'CAPTURA', 'EDICIÓN', 'LISTO'].entries()) {
      await expect(stages.nth(index)).toContainText(label);
    }
    // The plate describes stages; it must not claim one is running.
    await expect(page.locator('main [aria-current="step"]')).toHaveCount(0);
  });

  test('presents both plates over the guide render', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    const figure = page.locator('main img[src="/brand/onboarding-guide.webp"]');
    await expect(figure).toBeVisible();
    // Decorative: every word the render carries is repeated as real text in the
    // plates, so it must not be announced twice.
    await expect(figure).toHaveAttribute('alt', '');

    const plates = page.locator('main [data-slot="guide-plate"]');
    await expect(plates).toHaveCount(2);
    await expect(plates.nth(0)).toContainText('QUÉ PUEDES HACER');
    await expect(plates.nth(1)).toContainText('QUÉ PASA DESPUÉS');
  });

  test('overlays the plates only where the stage has room for them', async ({ page }) => {
    await page.setViewportSize({ width: 1600, height: 1000 });
    await gotoStudio(page, '/onboarding');
    const wide = await page.locator('main [data-slot="guide-plate"]').first().evaluate((node) => getComputedStyle(node).position);
    expect(wide).toBe('absolute');

    await page.setViewportSize({ width: 390, height: 900 });
    const narrow = await page.locator('main [data-slot="guide-plate"]').first().evaluate((node) => getComputedStyle(node).position);
    // Narrow: plates stand on their own. studio-panel is already relative.
    expect(narrow).toBe('relative');
  });

  test('does not duplicate the rail capture-readiness card', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    expect(await page.locator('[aria-label^="Captura:"]').count()).toBe(1);
    expect(await page.locator('main [aria-label^="Captura:"]').count()).toBe(0);
  });

  test('keeps the shell rather than hiding it from a first-run user', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    await expect(page.locator('[data-slot="sidebar-container"]').first()).toBeVisible();
    await expect(page.locator('header.shell-strip')).toBeVisible();
  });

  for (const width of VALIDATION_WIDTHS) {
    test(`doors stay reachable at ${width}px`, async ({ page }) => {
      await page.setViewportSize({ width, height: 900 });
      await gotoStudio(page, '/onboarding');
      for (const { href } of DOORS) {
        await expect(page.locator(`main a[href="${href}"]`)).toBeVisible();
      }
    });
  }
});
