import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

test.describe('Biblioteca', () => {
  test('does not gate the MP4 behind a cover candidate picker', async ({ page }) => {
    await gotoStudio(page, '/videos');
    await expect(page.getByText('Portada · elige candidata')).toHaveCount(0);
    await expect(page.getByText('Confirma una portada antes de marcar el pack listo para subir.')).toHaveCount(0);
    const mp4 = page.getByRole('button', { name: 'MP4' });
    if ((await mp4.count()) > 0) {
      await expect(mp4.first()).toBeEnabled();
    }
  });
});
