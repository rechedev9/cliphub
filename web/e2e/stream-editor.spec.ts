import { expect, test, type Page } from '@playwright/test';
import { fileURLToPath } from 'node:url';
import { gotoStudio } from './contract.ts';

/**
 * Stream editor (/streams/[id]). No orchestrator: one `ready` stream job with
 * a 30 s probe and one 12.4 s cut is served through the `/api/streams/*`
 * proxies stubbed at the network boundary, and every edit-plan PUT is
 * recorded so the tests can prove what autosave sent (or did not send).
 */
const JOB_ID = '5c1d7e2a-3b4f-4c6d-9e8f-0a1b2c3d4e5f';
const FACECAM_VARIANT = 'streamer-vertical-stack-40-60';
const OVERLAY_REQUIRED_ERROR = 'clip clip-1 text overlay text is required';
const AUTOSAVE_DEBOUNCE_MS = 500;

type Overlay = { text: string; position_y: number };
type Clip = {
  id: string;
  start_seconds: number;
  end_seconds: number;
  title: string;
  edit?: { text_overlays?: Overlay[] };
};
type Plan = { face_crop_reviewed: boolean; clips: Clip[] };

function editPlan(faceCropReviewed: boolean): Plan & Record<string, unknown> {
  return {
    schema_version: '1.1',
    variant: FACECAM_VARIANT,
    face_crop: { x: 0.62, y: 0.03, width: 0.34, height: 0.3 },
    face_crop_reviewed: faceCropReviewed,
    gameplay_crop: { x: 0, y: 0, width: 1, height: 1 },
    clips: [{ id: 'clip-1', start_seconds: 4, end_seconds: 16.4, title: 'Clutch 1v3' }],
    updated_at: '2026-09-01T10:00:00Z',
  };
}

function json(body: unknown, status = 200): { status: number; contentType: string; body: string } {
  return { status, contentType: 'application/json', body: JSON.stringify(body) };
}

type StreamStub = {
  /** Every edit-plan PUT body, in order. */
  puts: Plan[];
  /** Make the next PUTs fail with this 400 body, or pass null to accept them again. */
  rejectPuts(error: string | null): void;
  renderResult(mode: 'clips' | 'empty' | 'omitted'): void;
};

async function stubStreamJob(page: Page, faceCropReviewed: boolean, empty = false): Promise<StreamStub> {
  const puts: Plan[] = [];
  let putError: string | null = null;
  let plan = editPlan(faceCropReviewed);
  if (empty) plan.clips = [];
  let exported = false;
  let renderPolls = 0;
  let renderResult: 'clips' | 'empty' | 'omitted' = 'clips';
  await page.route(`**/api/streams/${JOB_ID}`, (route) =>
    route.fulfill(
      json({
        id: JOB_ID,
        status: 'ready',
        title: 'Clutch en Mirage',
        source_url: 'https://www.twitch.tv/donk/clip/ClutchOnMirage',
        probe: { width: 1920, height: 1080, duration_seconds: 30 },
        edit_plan: plan,
        created_at: '2026-09-01T10:00:00Z',
      }),
    ),
  );
  await page.route(`**/api/streams/${JOB_ID}/edit-plan`, (route) => {
    if (route.request().method() !== 'PUT') return route.fulfill(json(plan));
    const body = route.request().postDataJSON() as Plan;
    puts.push(body);
    if (putError !== null) return route.fulfill(json({ error: putError }, 400));
    plan = { ...body, updated_at: new Date().toISOString() };
    return route.fulfill(json(plan));
  });
  const mediaPath =
    process.env.STREAM_QA_SOURCE || fileURLToPath(new URL('./fixtures/stream-source.mp4', import.meta.url));
  await page.context().route(`**/api/streams/${JOB_ID}/renders/**`, (route) => {
    if (route.request().url().includes('/videos/')) return route.fulfill({ path: mediaPath, contentType: 'video/mp4' });
    if (route.request().method() === 'POST') {
      exported = true;
      renderPolls = 0;
      return route.fulfill(json({ status: 'queued' }));
    }
    if (!exported) return route.fulfill(json({ error: 'no render' }, 404));
    if (renderPolls++ === 0) return route.fulfill(json({ status: 'rendering' }));
    if (renderResult === 'omitted') return route.fulfill(json({ status: 'rendered' }));
    if (renderResult === 'empty') return route.fulfill(json({ status: 'rendered', videos: [] }));
    return route.fulfill(
      json({
        status: 'rendered',
        videos: plan.clips.map((c) => ({
          clip_id: c.id,
          title: c.title,
          key: `${c.id}.mp4`,
          duration_seconds: c.end_seconds - c.start_seconds,
        })),
      }),
    );
  });
  await page.route(`**/api/streams/${JOB_ID}/source`, (route) =>
    route.fulfill({ path: mediaPath, contentType: 'video/mp4' }),
  );
  await page.route('**/api/songs', (route) => route.fulfill(json({ songs: [] })));
  return {
    puts,
    rejectPuts(error) {
      putError = error;
    },
    renderResult(mode) {
      renderResult = mode;
    },
  };
}

const stepTitle = (page: Page, name: string) => page.getByRole('heading', { level: 2, name });
const railStep = (page: Page, name: RegExp) =>
  page.getByRole('navigation', { name: 'Pasos' }).getByRole('button', { name });
const autosaveStatus = (page: Page) => page.getByRole('navigation', { name: 'Pasos' }).getByRole('status');
const cta = (page: Page, name: string) => page.getByRole('button', { name, exact: true });
const briefCheckbox = (page: Page) => page.getByRole('checkbox', { name: 'He revisado y apruebo los ajustes' });

test.describe('stream editor', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('starts with moments and can revisit aspect without silently confirming or saving', async ({ page }) => {
    const stub = await stubStreamJob(page, false);
    await gotoStudio(page, `/streams/${JOB_ID}`);
    await expect(stepTitle(page, 'Elegir momentos')).toBeVisible();
    await expect(page.getByLabel('Inicio (s)', { exact: true })).toBeVisible();
    await cta(page, 'Continuar al aspecto →').click();
    await expect(stepTitle(page, 'Ajustar aspecto')).toBeVisible();
    await expect(page.getByLabel('Mover región de recorte del facecam')).toBeVisible();
    await expect(page.getByRole('region', { name: 'Timeline de la fuente' })).toBeVisible();
    await cta(page, 'Atrás').click();
    await expect(stepTitle(page, 'Elegir momentos')).toBeVisible();
    await page.waitForTimeout(AUTOSAVE_DEBOUNCE_MS * 2);
    expect(stub.puts).toHaveLength(0);
  });

  test('confirms the camera, handles pending renders, then previews and offers a titled download', async ({ page }) => {
    const stub = await stubStreamJob(page, false);
    await gotoStudio(page, `/streams/${JOB_ID}`);
    await page.getByLabel('Título del corte 01').fill('DALE NIÑO !');
    await cta(page, 'Continuar al aspecto →').click();
    await page.getByRole('button', { name: 'Confirmar cámara y continuar', exact: true }).click();
    await expect(stepTitle(page, 'Revisar y exportar')).toBeVisible();
    await expect(briefCheckbox(page)).toHaveCount(0);
    await expect.poll(() => stub.puts.at(-1)?.face_crop_reviewed).toBe(true);
    await page.getByText('Detalles de la exportación', { exact: true }).click();
    await expect(page.getByRole('definition').filter({ hasText: 'de salida aprox.' })).toHaveText(
      '1 clip · 0:12 de salida aprox.',
    );
    await cta(page, 'Exportar 1 Short →').click();
    await expect(stepTitle(page, 'Guardar vídeos')).toBeVisible();
    await expect(page.getByLabel('Vídeo final')).toBeVisible();
    await expect(cta(page, 'Exportar 1 Short →')).toHaveCount(0);
    const downloadEvent = page.waitForEvent('download');
    await page.getByRole('link', { name: 'Guardar vídeo: DALE NIÑO !' }).last().click();
    const download = await downloadEvent;
    expect(download.suggestedFilename()).toBe('DALE NIÑO !.mp4');
    // Chromium downloads bypass route mocks. Check the UI's filename and URL
    // here; the real file transfer and Explorer reveal are verified in Studio.
    expect(download.url()).toContain(`/api/streams/${JOB_ID}/renders/`);
    await download.cancel();
    await expect(cta(page, 'Abrir YouTube Studio')).toBeVisible();
    await railStep(page, /Elegir momentos/).click();
    await page.getByLabel('Título del corte 01').fill('Versión nueva');
    await railStep(page, /Guardar vídeos/).click();
    await expect(cta(page, 'Exportar 1 Short →')).toBeEnabled();
    await expect(page.getByRole('link', { name: /Guardar vídeo:/ })).toHaveCount(0);
  });

  for (const mode of ['empty', 'omitted'] as const) {
    test(`a completed render with ${mode} videos can be retried without editing the plan`, async ({ page }) => {
      const errors: string[] = [];
      page.on('pageerror', (error) => errors.push(error.message));
      const stub = await stubStreamJob(page, true);
      stub.renderResult(mode);
      await gotoStudio(page, `/streams/${JOB_ID}`);
      await cta(page, 'Continuar al aspecto →').click();
      await cta(page, 'Revisar Shorts →').click();
      await cta(page, 'Exportar 1 Short →').click();
      await expect(stepTitle(page, 'Guardar vídeos')).toBeVisible();
      await expect(
        page.getByText('No se generó ningún vídeo. Vuelve a exportar para intentarlo de nuevo.'),
      ).toBeVisible();
      await expect(page.getByText('Tus vídeos están listos.', { exact: false })).toHaveCount(0);
      await expect(page.getByRole('link', { name: /Guardar vídeo:/ })).toHaveCount(0);
      await expect(cta(page, 'Abrir YouTube Studio')).toHaveCount(0);
      expect(errors).toEqual([]);
      stub.renderResult('clips');
      await cta(page, 'Exportar 1 Short →').click();
      await expect(page.getByLabel('Vídeo final')).toBeVisible();
      await expect(page.getByRole('link', { name: 'Guardar vídeo: Clutch 1v3' })).toHaveCount(2);
      expect(errors).toEqual([]);
    });
  }

  test('both download buttons use the selected untitled Short number', async ({ page }) => {
    await stubStreamJob(page, true);
    await gotoStudio(page, `/streams/${JOB_ID}`);
    await cta(page, 'Nuevo momento').click();
    await page.getByLabel('Inicio (s)', { exact: true }).fill('20');
    await page.getByLabel('Fin (s)', { exact: true }).fill('23');
    await page.getByLabel('Fin (s)', { exact: true }).blur();
    await cta(page, 'Añadir este momento').click();
    await page.getByLabel('Título del corte 02').fill('');
    await cta(page, 'Continuar al aspecto →').click();
    await cta(page, 'Revisar Shorts →').click();
    await cta(page, 'Exportar 2 Shorts →').click();
    await expect(page.getByLabel('Vídeo final')).toBeVisible();
    await page.getByRole('button', { name: 'Short 2 · 0:03', exact: true }).click();
    await expect(stepTitle(page, 'Short 2')).toBeVisible();
    const downloads = page.getByRole('link', { name: 'Guardar vídeo: Short 2', exact: true });
    await expect(downloads).toHaveCount(2);
    for (const link of await downloads.all()) {
      await expect(link).toHaveAttribute('download', 'Short 2.mp4');
      expect(await link.getAttribute('href')).toBe(await page.getByLabel('Vídeo final').getAttribute('src'));
    }
    await page.getByRole('button', { name: 'Clutch 1v3 · 0:12', exact: true }).click();
    await expect(page.getByRole('link', { name: 'Guardar vídeo: Clutch 1v3', exact: true })).toHaveCount(2);
  });

  test('"Añadir texto" keeps a blank overlay local until text is typed, and a cleared text leaves the plan', async ({
    page,
  }) => {
    const stub = await stubStreamJob(page, true);
    await gotoStudio(page, `/streams/${JOB_ID}`);

    await expect(stepTitle(page, 'Elegir momentos')).toBeVisible();
    await page.getByText('Texto, velocidad y audio · opcional', { exact: true }).click();
    await page.getByRole('button', { name: '+ Texto' }).click();
    await page.getByRole('button', { name: 'Añadir texto' }).click();
    const textField = page.getByLabel('Texto', { exact: true });
    await expect(textField).toBeVisible();
    await expect(textField).toHaveAttribute('aria-invalid', 'true');
    await page.waitForTimeout(AUTOSAVE_DEBOUNCE_MS * 2);
    expect(stub.puts).toHaveLength(0);

    await textField.fill('NICE SHOT');
    await expect.poll(() => stub.puts.at(-1)?.clips[0]?.edit?.text_overlays?.[0]?.text).toBe('NICE SHOT');
    await expect(autosaveStatus(page)).toHaveText('Borrador guardado en este PC');

    await textField.fill('');
    await expect.poll(() => stub.puts.length).toBeGreaterThan(1);
    expect(stub.puts.at(-1)?.clips[0]?.edit).toBeUndefined();
    await expect(textField).toBeVisible();
    await expect(autosaveStatus(page)).toHaveText('Borrador guardado en este PC');
  });

  test('a rejected autosave PUT shows the failed state, then clears once a save lands', async ({ page }) => {
    const stub = await stubStreamJob(page, true);
    stub.rejectPuts(OVERLAY_REQUIRED_ERROR);
    await gotoStudio(page, `/streams/${JOB_ID}`);

    await page.getByLabel('Título del corte 01').fill('Clutch 1v3 final');
    await expect(autosaveStatus(page)).toHaveText('Borrador local · guardado pendiente');
    await expect(page.getByRole('alert').filter({ hasText: OVERLAY_REQUIRED_ERROR })).toBeVisible();
    expect(stub.puts.length).toBeGreaterThan(0);

    stub.rejectPuts(null);
    await page.getByLabel('Título del corte 01').fill('Clutch 1v3 final ronda 30');
    await expect(autosaveStatus(page)).toHaveText('Borrador guardado en este PC');
    await expect(page.getByRole('alert').filter({ hasText: OVERLAY_REQUIRED_ERROR })).toHaveCount(0);
  });
});

test('plays and seeks the original before any moments, then uses the whole short source', async ({ page }) => {
  const stub = await stubStreamJob(page, false, true);
  await gotoStudio(page, `/streams/${JOB_ID}`);
  const decoder = page.locator('video[data-stream-frame="shared-decoder"]');
  await expect(decoder).toHaveCount(1);
  await expect(page.locator('audio')).toHaveCount(0);
  await cta(page, 'Reproducir vídeo original').click();
  await expect
    .poll(() => decoder.evaluate((element) => {
      if (!(element instanceof HTMLVideoElement)) throw new Error('expected the shared video decoder');
      return element.currentTime;
    }))
    .toBeGreaterThan(0.5);
  await cta(page, 'Pausar').click();
  const timeline = page.getByRole('slider', { name: 'Posición en el vídeo original' });
  await timeline.fill('5');
  await expect(page.getByLabel('Tiempo de reproducción')).toContainText('0:05');
  expect(stub.puts).toHaveLength(0);
  await cta(page, 'Marcar inicio aquí').click();
  await expect(page.getByLabel('Inicio (s)', { exact: true })).toHaveValue('5');
  await timeline.fill('8');
  await cta(page, 'Marcar final aquí').click();
  await expect(page.getByLabel('Fin (s)', { exact: true })).toHaveValue('8');
  expect(stub.puts).toHaveLength(0);
  await cta(page, 'Usar vídeo completo').click();
  await expect.poll(() => stub.puts.at(-1)?.clips.length).toBe(1);
  expect(stub.puts.at(-1)?.clips[0]).toMatchObject({ start_seconds: 0, end_seconds: 30 });
  await expect(page.getByLabel('Título del corte 01')).toHaveValue('Clutch en Mirage');
});
test('a selected Short stops at its end and selecting another exposes only its controls', async ({ page }) => {
  const stub = await stubStreamJob(page, true);
  await gotoStudio(page, `/streams/${JOB_ID}`);
  await page.getByLabel('Inicio (s)', { exact: true }).fill('0');
  await page.getByLabel('Fin (s)', { exact: true }).fill('1');
  await page.getByLabel('Fin (s)', { exact: true }).blur();
  await cta(page, 'Nuevo momento').click();
  await page.getByLabel('Inicio (s)', { exact: true }).fill('5');
  await page.getByLabel('Fin (s)', { exact: true }).fill('8');
  await page.getByLabel('Fin (s)', { exact: true }).blur();
  await cta(page, 'Añadir este momento').click();
  await page.getByRole('list', { name: 'Tus momentos' }).getByRole('button').first().click();
  const decoder = page.locator('video[data-stream-frame="shared-decoder"]');
  // This clip lasts only one second. Under parallel media tests the pause
  // button can disappear before Playwright samples it; observe actual playback
  // before clicking, then still require the final stopped controls and clock.
  await decoder.evaluate((element) => {
    element.addEventListener('playing', () => { (element as HTMLElement).dataset.e2ePlayed = 'true'; }, { once: true });
  });
  await cta(page, 'Ver este Short').click();
  await expect(decoder).toHaveAttribute('data-e2e-played', 'true');
  await expect(cta(page, 'Reproducir este Short')).toBeVisible();
  await expect(page.getByLabel('Tiempo de reproducción')).toContainText('0:01');
  await expect(decoder).toHaveCount(1);
  await expect(page.locator('audio')).toHaveCount(0);
  const stopped = await decoder.evaluate((element) => {
    if (!(element instanceof HTMLVideoElement)) throw new Error('expected the shared video decoder');
    return { paused: element.paused, seconds: element.currentTime };
  });
  expect(stopped.paused).toBe(true);
  expect(stopped.seconds).toBeGreaterThanOrEqual(0.9);
  await expect(page.getByLabel('Inicio (s)', { exact: true })).toHaveCount(1);
  await page.getByRole('button', { name: /Seleccionar el corte 2:/ }).click();
  await expect(page.getByLabel('Inicio (s)', { exact: true })).toHaveValue('5');
  await expect.poll(() => stub.puts.at(-1)?.clips.length).toBe(2);
  await page.getByRole('button', { name: 'Quitar corte 02' }).click();
  await expect(page.getByRole('list', { name: 'Tus momentos' }).getByRole('button')).toHaveCount(1);
  await cta(page, 'Deshacer').click();
  await expect(page.getByRole('list', { name: 'Tus momentos' }).getByRole('button')).toHaveCount(2);
});
for (const width of [390, 768, 1266, 1920]) {
  test(`editor layout at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await stubStreamJob(page, false, true);
    await gotoStudio(page, `/streams/${JOB_ID}`);
    await expect(stepTitle(page, 'Elegir momentos')).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    const media = page.getByRole('region', { name: 'Monitor', exact: true });
    if (width >= 1266) {
      const bounds = await media.boundingBox();
      expect(bounds!.y).toBeLessThan(240);
      await expect(cta(page, 'Añadir este momento')).toBeInViewport();
      await expect(cta(page, 'Continuar al aspecto →')).toBeInViewport();
    }
    await page.screenshot({ path: testInfo.outputPath(`moments-${width}.png`), fullPage: true });
    await cta(page, 'Usar vídeo completo').click();
    await cta(page, 'Continuar al aspecto →').click();
    await expect(page.getByLabel('Mover región de recorte del facecam')).toBeVisible();
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`aspect-${width}.png`), fullPage: true });
  });
}
