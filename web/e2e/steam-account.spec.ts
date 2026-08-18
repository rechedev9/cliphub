import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

const EMPTY_ACCOUNT = {
  steamId: '',
  authCodeSet: false,
  apiKeySet: false,
  knownCode: '',
  historyConfigured: false,
  gcConfigured: false,
  matches: [],
};

test.describe('Steam account settings', () => {
  test('collects the revocable history credentials, not the password', async ({ page }) => {
    const puts: unknown[] = [];
    await page.route('**/api/steam/account', async (route) => {
      if (route.request().method() === 'PUT') {
        puts.push(route.request().postDataJSON());
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...EMPTY_ACCOUNT,
            steamId: '76561198000000001',
            authCodeSet: true,
            apiKeySet: true,
            historyConfigured: true,
          }),
        });
        return;
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(EMPTY_ACCOUNT),
      });
    });

    await gotoStudio(page, '/settings');
    await expect(page.getByRole('heading', { name: 'Steam' })).toBeVisible();
    await expect(page.getByLabel('Contraseña')).toHaveCount(0);
    await expect(page.getByLabel('Steam Guard')).toHaveCount(0);
    await page.getByLabel('SteamID64').fill('76561198000000001');
    await page.getByLabel('Código de autenticación').fill('AAAAA-BBBBB-CCCCC');
    await page.getByLabel('Clave de la Web API').fill('0123456789ABCDEF');
    await page.getByRole('button', { name: 'GUARDAR' }).click();
    await expect(page.getByText('Historial conectado')).toBeVisible();
    expect(puts).toEqual([{
      steamId: '76561198000000001',
      authCode: 'AAAAA-BBBBB-CCCCC',
      apiKey: '0123456789ABCDEF',
      knownCode: '',
    }]);
  });
});
