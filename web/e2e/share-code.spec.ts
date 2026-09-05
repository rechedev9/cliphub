import { expect, test, type Page } from '@playwright/test';
import { VALIDATION_WIDTHS, gotoStudio } from './contract.ts';

const STEAM_HELP_HREF =
  'https://help.steampowered.com/en/wizard/HelpWithGameIssue/?appid=730&issueid=128';

const WELL_FORMED_CODE = 'CSGO-GADqf-jjyJ8-cSP2r-smZRo-TO2xK';

// 64-bit ids kept as strings end to end; a JS number would corrupt them.
const MATCH_ID = '3230642215713767581';
const OUTCOME_ID = '3230642252279119992';

type StubResponse = { status: number; body: unknown };

async function stubShareCode(page: Page, response: StubResponse): Promise<{ bodies: unknown[] }> {
  const bodies: unknown[] = [];
  await page.route('**/api/steam/sharecode', async (route) => {
    bodies.push(route.request().postDataJSON());
    await route.fulfill({
      status: response.status,
      contentType: 'application/json',
      body: JSON.stringify(response.body),
    });
  });
  return { bodies };
}

async function submitCode(page: Page, code: string): Promise<void> {
  await page.getByLabel('Código de partida').fill(code);
  await page.getByRole('button', { name: 'COMPROBAR' }).click();
}

test.describe('share code door', () => {
  test('explains where the code comes from before asking for it', async ({ page }) => {
    // The three retrieval steps are shown up front, not hidden behind an error.
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    const section = page.locator('main section', {
      hasText: '¿YA TIENES EL CÓDIGO DE UNA PARTIDA?',
    });
    const steps = section.locator('ol li');
    await expect(steps).toHaveCount(3);
    await expect(section.locator('ol')).toBeVisible();
    await expect(section.locator('ol')).toContainText('Tus partidas');
    await expect(section.locator('ol')).toContainText('CSGO-');
  });

  test('links the official Steam page and never opens it unsafely', async ({ page }) => {
    // target="_blank" without rel is the reverse-tabnabbing defect this guards.
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    const anchor = page.locator(`main a[href="${STEAM_HELP_HREF}"]`);
    await expect(anchor).toBeVisible();
    await expect(anchor).toHaveAttribute('target', '_blank');
    const rel = await anchor.getAttribute('rel');
    expect(rel ?? '').toContain('noreferrer');
  });

  test('rejects a malformed code locally, without a round trip', async ({ page }) => {
    // A malformed code gets a visible field error, no success tag, no request.
    const stub = await stubShareCode(page, { status: 200, body: {} });
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    await submitCode(page, 'CSGO-nope');
    await expect(page.locator('main [data-slot="field-error"]')).toBeVisible();
    await expect(page.getByText('Código válido')).toHaveCount(0);
    expect(stub.bodies).toHaveLength(0);
  });

  test('a decoded code is valid but points at Ajustes for the download', async ({ page }) => {
    const stub = await stubShareCode(page, {
      status: 200,
      body: { status: 'decoded', matchId: MATCH_ID, outcomeId: OUTCOME_ID, tokenId: 31463 },
    });
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    await submitCode(page, WELL_FORMED_CODE);
    await expect(page.getByText('Código válido')).toBeVisible();
    const live = page.locator('main [aria-live="polite"]');
    await expect(live).toContainText(MATCH_ID);
    await expect(live).toContainText(/steam/i);
    await expect(live.getByRole('link', { name: 'Ajustes' })).toBeVisible();
    await expect(page.getByText(/descargando|bajando/i)).toHaveCount(0);
    expect(stub.bodies).toEqual([{ code: WELL_FORMED_CODE }]);
  });

  test('a resolved code offers to enqueue the demo', async ({ page }) => {
    await stubShareCode(page, {
      status: 200,
      body: {
        status: 'resolved',
        matchId: MATCH_ID,
        outcomeId: OUTCOME_ID,
        tokenId: 31463,
        demoUrl: 'http://replay1.valve.net/730/demo.dem.bz2',
      },
    });
    const imported: unknown[] = [];
    await page.route('**/api/steam/import', async (route) => {
      imported.push(route.request().postDataJSON());
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ id: '11111111-1111-4111-8111-111111111111', status: 'queued', matchId: MATCH_ID }),
      });
    });
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    await submitCode(page, WELL_FORMED_CODE);
    await expect(page.getByText('Código válido')).toBeVisible();
    await page.getByRole('button', { name: 'DESCARGAR DEMO' }).click();
    await expect(page).toHaveURL(/\/clips\/nueva\?job=11111111-1111-4111-8111-111111111111&formato=short$/);
    expect(imported).toEqual([{ code: WELL_FORMED_CODE }]);
  });

  test('download without a Steam session asks for the login', async ({ page }) => {
    await stubShareCode(page, {
      status: 200,
      body: { status: 'decoded', matchId: MATCH_ID, outcomeId: OUTCOME_ID, tokenId: 31463 },
    });
    await page.route('**/api/steam/import', async (route) => {
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'steam_credentials_required', error: 'Steam login is required' }),
      });
    });
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    await submitCode(page, WELL_FORMED_CODE);
    await page.getByRole('button', { name: 'DESCARGAR DEMO' }).click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByRole('dialog').getByLabel('Usuario de Steam')).toBeVisible();
    await expect(page.getByRole('dialog').getByLabel('Contraseña')).toBeVisible();
    await expect(page.getByRole('dialog').getByLabel('Steam Guard')).toBeVisible();
  });

  test('a 400 shows the upstream reason as a field error', async ({ page }) => {
    await stubShareCode(page, {
      status: 400,
      body: { code: 'invalid_share_code', message: 'Ese código no corresponde a ninguna partida.' },
    });
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    await submitCode(page, WELL_FORMED_CODE);
    await expect(page.locator('main [data-slot="field-error"]')).toContainText(
      'Ese código no corresponde a ninguna partida.',
    );
    await expect(page.getByText('Código válido')).toHaveCount(0);
  });

  test('a 503 blames the local service, never the code', async ({ page }) => {
    await stubShareCode(page, {
      status: 503,
      body: { error: 'analysis service unavailable', code: 'service_unavailable' },
    });
    await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
    await submitCode(page, WELL_FORMED_CODE);
    const live = page.locator('main [aria-live="polite"]');
    await expect(live).toContainText('El servicio local de ClipHub no está en marcha');
    await expect(page.locator('main [data-slot="field-error"]')).toHaveCount(0);
    await expect(page.getByText('Código válido')).toHaveCount(0);
  });

  for (const width of VALIDATION_WIDTHS) {
    test(`stays usable without overflow at ${width}px`, async ({ page }) => {
      // The form works at every validated width with no x-scroll.
      await page.setViewportSize({ width, height: 900 });
      await gotoStudio(page, '/clips/nueva');
    await page.getByRole('tab', { name: 'Importar desde Steam' }).click();
      await expect(page.getByLabel('Código de partida')).toBeVisible();
      await expect(page.getByRole('button', { name: 'COMPROBAR' })).toBeVisible();
      const overflow = await page.evaluate(() => {
        const root = document.documentElement;
        const offenders = [...document.querySelectorAll<HTMLElement>('main *')]
          .filter((node) => node.getBoundingClientRect().right > root.clientWidth + 1)
          .slice(0, 4)
          .map((node) => {
            const box = node.getBoundingClientRect();
            return `${node.tagName.toLowerCase()}.${node.className.slice(0, 50)} right=${Math.round(box.right)}`;
          });
        return { scrollWidth: root.scrollWidth, clientWidth: root.clientWidth, offenders };
      });
      expect(
        overflow.scrollWidth,
        `overflows by ${overflow.scrollWidth - overflow.clientWidth}px: ${overflow.offenders.join(' | ')}`,
      ).toBeLessThanOrEqual(overflow.clientWidth);
    });
  }
});
