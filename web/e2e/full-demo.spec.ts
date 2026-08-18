import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

test.describe('Full demo to video', () => {
  test('is a numbered production section', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    const key = page.locator('[data-slot="sidebar"] a[href="/full-demo"]');
    await expect(key).toBeVisible();
    await expect(key).toContainText('12');
    await expect(key).toContainText('Full demo to video');
  });

  test('states the locked 16:9 recap contract', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    await expect(page.getByRole('heading', { name: 'FULL DEMO TO VIDEO' })).toBeVisible();
    await expect(page.getByText('Rondas completas')).toBeVisible();
    await expect(page.getByText('Nativo CS2 (radar, vida, killfeed)')).toBeVisible();
    await expect(page.getByText('Horizontal 16:9 · 1920×1080')).toBeVisible();
  });

  test('empty state hands back to Inicio, not a competing door', async ({ page }) => {
    await gotoStudio(page, '/full-demo');
    const empty = page.locator('section[aria-label="No hay demos para forjar"]');
    await expect(empty).toBeVisible();
    await expect(empty.locator('a')).toHaveCount(1);
    await expect(empty.locator('a')).toHaveAttribute('href', '/onboarding');
  });

  test('a missing job does not offer FORJAR', async ({ page }) => {
    await gotoStudio(page, '/full-demo/11111111-1111-4111-8111-111111111111');
    await expect(page.getByText(/Servicio local sin conexión|Demo no encontrada/)).toBeVisible();
    await expect(page.getByRole('button', { name: 'FORJAR REEL' })).toHaveCount(0);
  });
});
