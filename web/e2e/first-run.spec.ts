import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

/** First-run contract: Inicio owns the doors; Partidas hands over instead of competing. */
test.describe('first run', () => {
  // The empty state routes a first-run user to Inicio, and nowhere else.
  test('Partidas offers exactly one way out of the empty state', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const empty = page.locator('section[aria-label="Aún no hay partidas"]');
    await expect(empty).toBeVisible();
    const links = empty.locator('a');
    await expect(links).toHaveCount(1);
    await expect(links).toHaveAttribute('href', '/onboarding');
  });

  // Two screens competing to be the first screen is the thing this removes:
  // the doors live on Inicio, so Partidas must not re-offer them.
  test('Partidas no longer duplicates the entry doors', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const empty = page.locator('section[aria-label="Aún no hay partidas"]');
    await expect(empty).toBeVisible();
    await expect(empty.locator('a[href="/upload"]')).toHaveCount(0);
    await expect(empty.locator('a[href="/streams"]')).toHaveCount(0);
  });

  // Inicio keeps its three doors; no other screen lists all of them.
  test('Inicio is the only screen listing every door', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    for (const href of ['/upload', '/streams', '/players']) {
      await expect(page.locator(`main a[href="${href}"]`)).toHaveCount(1);
    }
    await gotoStudio(page, '/matches');
    await expect(page.locator('main a[href="/players"]')).toHaveCount(0);
  });
});
