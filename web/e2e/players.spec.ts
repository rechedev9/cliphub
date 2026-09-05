import { expect, test, type Page } from '@playwright/test';
import type { FaceitFollowedPlayer, FaceitMatch } from '../lib/api/faceit.ts';
import { gotoStudio, VALIDATION_WIDTHS } from './contract.ts';

const PLAYERS: FaceitFollowedPlayer[] = [
  ['donk666', 4686], ['-SYPHO', 4431], ['dem0nnnnnnn', 4348], ['1769-', 4247],
  ['CEMEN_BAKIN', 4187], ['nipl', 4213], ['whuhurt', 4157], ['bluewh1te', 4148], ['73ddd', 4143], ['em0k1d', 4107],
].map(([nickname, elo], index) => ({ id: `player-${index}`, nickname: String(nickname), elo: Number(elo),
  skill_level: 10, profile_url: `https://www.faceit.com/en/players/${nickname}`,
  steam_id64: '76561198386265483', seeded: true }));

const MATCHES: FaceitMatch[] = Array.from({ length: 20 }, (_, index) => ({
  id: `match-${index}`, room_url: `https://www.faceit.com/en/cs2/room/match-${index}`,
  finished_at: '2026-09-04T12:00:00Z', score: { for: 13, against: 4 },
  stats: { map: index % 2 === 0 ? 'de_anubis' : 'de_mirage', result: index % 5 === 4 ? 'loss' : 'win',
    kills: 20, deaths: 10, assists: 4, kd_ratio: 2, adr: 108, headshots_percent: 68 },
}));

async function stubFaceit(page: Page): Promise<void> {
  let followed = [...PLAYERS];
  await page.route('**/api/faceit/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (path.endsWith('/avatar')) { await route.fulfill({ status: 404 }); return; }
    if (path.endsWith('/matches')) {
      await route.fulfill({ json: { matches: path.includes('player-1/') ? MATCHES.slice(0, 2) : MATCHES } }); return;
    }
    if (route.request().method() === 'DELETE') {
      followed = followed.filter((player) => player.id !== path.split('/').at(-1));
      await route.fulfill({ status: 204 }); return;
    }
    if (path.endsWith('/followed')) {
      if (route.request().method() === 'POST') {
        const player: FaceitFollowedPlayer = { id: 'new-player', nickname: 'ropz', elo: 4900, seeded: true,
          skill_level: 10, profile_url: 'https://www.faceit.com/en/players/ropz' };
        followed = [...followed, player];
        await route.fulfill({ json: { player } }); return;
      }
      await route.fulfill({ json: { enabled: true, players: followed } }); return;
    }
    const player = followed.find((candidate) => candidate.nickname === url.searchParams.get('nickname'));
    await route.fulfill({ json: { player } });
  });
}

test('search, ordering, player selection and follow management use the real UI flow', async ({ page }) => {
  await stubFaceit(page);
  await gotoStudio(page, '/players');
  const rail = page.getByRole('navigation', { name: 'Jugadores seguidos' });
  await expect(rail.getByRole('button').first()).toContainText('donk666');
  await expect(rail.getByRole('button').nth(4)).toContainText('nipl');
  await page.getByRole('textbox', { name: 'Buscar jugador seguido' }).fill('sypho');
  await expect(rail.getByRole('button')).toHaveCount(1);
  await rail.getByRole('button').click();
  await expect(page.getByRole('region', { name: 'Perfil de -SYPHO' })).toBeVisible();
  await expect(page.getByText('Mostrando 1–2 de 2 partidas')).toBeVisible();
  await page.getByRole('textbox', { name: 'Buscar jugador seguido' }).clear();
  await page.getByRole('combobox', { name: 'Ordenar jugadores' }).click();
  await page.getByRole('option', { name: 'A–Z', exact: true }).click();
  await expect(rail.getByRole('button').first()).toContainText('-SYPHO');
  await page.getByRole('textbox', { name: 'Nick o URL de FACEIT', exact: true }).fill('ropz');
  await page.getByRole('button', { name: 'Seguir jugador', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Perfil de ropz' })).toBeVisible();
  await page.getByRole('button', { name: 'Opciones de ropz' }).click();
  await page.getByRole('menuitem', { name: 'Dejar de seguir a ropz' }).click();
  await expect(rail.getByRole('button', { name: /ropz/ })).toHaveCount(0);
  await expect(page.getByRole('region', { name: 'Perfil de donk666' })).toBeVisible();
});

test('match filters reset pagination without changing the overall performance summary', async ({ page }) => {
  await stubFaceit(page);
  await gotoStudio(page, '/players');
  await expect(page.getByText('Mostrando 1–7 de 20 partidas')).toBeVisible();
  await page.getByRole('button', { name: 'Página 3', exact: true }).click();
  await expect(page.getByText('Mostrando 15–20 de 20 partidas')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Página siguiente' })).toBeDisabled();
  await page.getByRole('combobox', { name: 'Filtrar por mapa' }).click();
  await page.getByRole('option', { name: 'Anubis', exact: true }).click();
  await page.getByRole('combobox', { name: 'Filtrar por resultado' }).click();
  await page.getByRole('option', { name: 'Derrotas', exact: true }).click();
  await expect(page.getByText('Mostrando 1–2 de 2 partidas')).toBeVisible();
  await expect(page.getByText('16 victorias · 4 derrotas')).toBeVisible();
  await expect(page.getByRole('table').getByRole('link', { name: 'Abrir sala FACEIT de Anubis' })).toHaveCount(2);
  await expect(page.getByRole('table').getByRole('link').first()).toHaveAttribute('href', MATCHES[4]?.room_url ?? '');
  await expect(page.getByRole('link', { name: 'Subir demo', exact: true })).toHaveAttribute('href', '/upload');
  await page.getByRole('combobox', { name: 'Filtrar por resultado' }).click();
  await page.getByRole('option', { name: 'Sin resultado', exact: true }).click();
  await expect(page.getByRole('table')).toContainText('No hay partidas con estos filtros.');
  await page.getByRole('button', { name: 'Restablecer filtros' }).click();
  await expect(page.getByText('Mostrando 1–7 de 20 partidas')).toBeVisible();
});

test('history errors can be retried and refreshed without navigating away', async ({ page }) => {
  await stubFaceit(page);
  let fail = true;
  let requests = 0;
  await page.route('**/api/faceit/players/*/matches?*', async (route) => {
    requests += 1;
    await route.fulfill(fail ? { status: 503, json: { error: 'Unavailable' } } : { json: { matches: MATCHES } });
  });
  await gotoStudio(page, '/players');
  await expect(page.getByRole('heading', { name: 'No se pudieron cargar las partidas' })).toBeVisible();
  fail = false;
  await page.getByRole('button', { name: 'Reintentar', exact: true }).click();
  await expect(page.getByText('Mostrando 1–7 de 20 partidas')).toBeVisible();
  const previousRequests = requests;
  await page.getByRole('button', { name: 'Actualizar partidas' }).click();
  await expect.poll(() => requests).toBeGreaterThan(previousRequests);
});

test('unconfigured and empty states keep a useful next action', async ({ page }) => {
  await stubFaceit(page);
  let enabled = false;
  await page.route('**/api/faceit/followed', async (route) => {
    await route.fulfill({ json: { enabled, players: [] } });
  });
  await gotoStudio(page, '/players');
  await expect(page.getByRole('heading', { name: 'FACEIT no está configurado' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Cargar una demo' })).toBeVisible();
  enabled = true;
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Aún no sigues a nadie' })).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Nick o URL de FACEIT', exact: true })).toBeEnabled();
});

for (const width of VALIDATION_WIDTHS) {
  test(`populated players workspace fits ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 1080 });
    await stubFaceit(page);
    await gotoStudio(page, '/players');
    await expect(page.getByRole('heading', { name: 'Historial de partidas' })).toBeVisible();
    await expect(page.getByRole('link', { name: /Abrir sala FACEIT/ })).toHaveCount(7);
    const geometry = await page.evaluate(() => ({
      width: document.documentElement.clientWidth, scrollWidth: document.documentElement.scrollWidth,
      mainRight: document.querySelector('main')?.getBoundingClientRect().right,
    }));
    expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.width);
    if (width === 1920) expect(geometry.mainRight).toBe(width);
    await page.screenshot({ path: testInfo.outputPath(`players-${width}.png`), fullPage: true });
  });
}
