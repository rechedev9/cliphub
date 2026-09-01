import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

/** First-run contract: Inicio owns the doors; Demos hands over instead of competing. */
test.describe('first run', () => {
  // The empty state routes a first-run user to Inicio, and nowhere else.
  test('Demos offers exactly one way out of the empty state', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const empty = page.locator('section[aria-label="Aún no hay demos"]');
    await expect(empty).toBeVisible();
    const links = empty.locator('a');
    await expect(links).toHaveCount(1);
    await expect(links).toHaveAttribute('href', '/onboarding');
  });

  // Two screens competing to be the first screen is the thing this removes:
  // the doors live on Inicio, so Demos must not re-offer them.
  test('Demos no longer duplicates the entry doors', async ({ page }) => {
    await gotoStudio(page, '/matches');
    const empty = page.locator('section[aria-label="Aún no hay demos"]');
    await expect(empty).toBeVisible();
    await expect(empty.locator('a[href="/upload"]')).toHaveCount(0);
    await expect(empty.locator('a[href="/streams"]')).toHaveCount(0);
  });

  // Inicio → Crea Shorts must not dump a first-run user on empty Demos.
  test('upload chrome returns to Inicio, not empty Demos', async ({ page }) => {
    await gotoStudio(page, '/onboarding');
    await page.locator('main a[href="/upload"]').click();
    await expect(page).toHaveURL(/\/upload$/);

    const wordmark = page.getByRole('link', { name: 'Inicio de ClipHub' });
    await expect(wordmark).toHaveAttribute('href', '/onboarding');
    const back = page.getByRole('link', { name: 'Volver' });
    await expect(back).toHaveAttribute('href', '/onboarding');

    await wordmark.click();
    await expect(page).toHaveURL(/\/onboarding$/);
    await expect(page.locator('section[aria-label="Aún no hay demos"]')).toHaveCount(0);

    await page.locator('main a[href="/upload"]').click();
    await expect(page).toHaveURL(/\/upload$/);
    await back.click();
    await expect(page).toHaveURL(/\/onboarding$/);
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
