import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

const JOB = '11111111-1111-4111-8111-111111111111';
const CODE = 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK';

test('a new user can discover the inputs and reach capture requirements', async ({ page }) => {
  await page.route('**/api/demos/jobs', route => route.fulfill({ json: { jobs: [] } }));
  await gotoStudio(page, '/clips');
  await expect(page.getByRole('heading', { name: 'De la demo a tu vídeo' })).toBeVisible();
  await page.getByText('¿Qué es una demo y dónde la consigo?', { exact: true }).click();
  await expect(page.getByText('No necesitas conectar Steam', { exact: false })).toBeVisible();
  await page.getByRole('link', { name: 'Revisar requisitos de grabación' }).click();
  await expect(page).toHaveURL(/\/settings#capture$/);
  await page.locator('main').getByRole('button', { name: /Grabación de demos/ }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('heading', { name: 'Requisitos de grabación' })).toBeVisible();
  await expect(dialog.getByText(/Los clips de stream/)).toBeVisible();
  await expect(dialog.getByText('ZV_HLAE_PATH', { exact: true })).not.toBeVisible();
});

test('file and Steam inputs are separate and keep the chosen format', async ({ page }) => {
  await gotoStudio(page, '/clips/nueva?formato=full');
  await expect(page.getByRole('tab', { name: 'Archivo en mi PC' })).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByRole('button', { name: 'Vídeo largo 16:9' })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByLabel('Elegir demos de CS2')).toBeEnabled();
  await expect(page.getByLabel('Código de partida')).toHaveCount(0);
  await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
  await expect(page.getByLabel('Código de partida')).toBeVisible();
  await page.getByRole('tab', { name: 'Archivo en mi PC' }).click();
  await expect(page.getByLabel('Elegir demos de CS2')).toBeEnabled();
  await expect(page).toHaveURL(/formato=full$/);
});

test('Steam import preserves long-video intent through the player handoff', async ({ page }) => {
  await page.route('**/api/steam/sharecode', route => route.fulfill({ json: { status: 'decoded', matchId: '3230642215713767581', outcomeId: '3230642252279119992', tokenId: 31463 } }));
  await page.route('**/api/steam/import', route => route.fulfill({ status: 201, json: { id: JOB, status: 'queued' } }));
  await page.route(`**/api/demos/${JOB}/status`, route => route.fulfill({ json: { status: 'scanned' } }));
  await page.route(`**/api/demos/${JOB}/roster`, route => route.fulfill({ json: { players: [{ steamid64: '76561198000000001', name: 'ropz', kills: 12, deaths: 8, assists: 4, team: 'CT' }] } }));
  await gotoStudio(page, '/clips/nueva?formato=full');
  await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
  await page.getByLabel('Código de partida').fill(CODE);
  await page.getByRole('button', { name: 'COMPROBAR' }).click();
  await page.getByRole('button', { name: 'DESCARGAR DEMO' }).click();
  await expect(page).toHaveURL(new RegExp(`/clips/${JOB}/nuevo\\?formato=full$`));
  await expect(page.locator('[aria-current="step"]')).toHaveText('2Elegir jugador');
  await page.getByRole('button', { name: 'Elegir jugador', exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/clips/nueva\\?job=${JOB}&formato=full$`));
  await expect(page.getByRole('button', { name: 'Continuar al vídeo largo' })).toBeEnabled();
});

test('disabled FACEIT offers a next step without requesting profiles, matches or avatars', async ({ page }) => {
  const requests: string[] = [];
  page.on('request', request => {
    if (request.url().includes('/api/faceit/players')) requests.push(request.url());
  });
  await page.route('**/api/faceit/followed', route => route.fulfill({ json: {
    enabled: false,
    players: [{ id: JOB, nickname: 'saved-player', profile_url: 'https://www.faceit.com/en/players/saved-player', seeded: true }],
  } }));
  await gotoStudio(page, '/players');
  await expect(page.getByRole('heading', { name: 'FACEIT no está configurado' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Cargar una demo' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Jugadores seguidos' })).toHaveCount(0);
  expect(requests).toEqual([]);
});

test('a failed archive read can be retried with the same input', async ({ page }) => {
  await gotoStudio(page, '/clips/nueva');
  const input = page.getByLabel('Elegir demos de CS2');
  await expect(input).toBeEnabled();
  await input.setInputFiles({ name: 'incomplete.zip', mimeType: 'application/zip', buffer: Buffer.from('broken archive') });
  await expect(page.locator('main [role="alert"]')).toBeVisible();
  await expect(input).toBeEnabled();
  await page.route('**/api/demos/scan', route => route.fulfill({ status: 400, json: { error: 'La demo está incompleta.', code: 'invalid_demo' } }));
  await input.setInputFiles({ name: 'retry.dem', mimeType: 'application/octet-stream', buffer: Buffer.from('HL2DEMO\0test') });
  await expect(page.locator('main [role="alert"]')).toContainText('No se pudo');
  await expect(input).toBeEnabled();
});
