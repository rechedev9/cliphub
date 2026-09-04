import { expect, test, type Page } from '@playwright/test';
import { gotoStudio } from './contract.ts';

/**
 * Stream editor (/streams/[id]). No orchestrator: one `ready` stream job with
 * a 60 s probe and one 12.4 s cut is served through the `/api/streams/*`
 * proxies stubbed at the network boundary, and every edit-plan PUT is
 * recorded so the tests can prove what autosave sent (or did not send).
 */
const JOB_ID = '5c1d7e2a-3b4f-4c6d-9e8f-0a1b2c3d4e5f';
const FACECAM_VARIANT = 'streamer-vertical-stack-40-60';
const OVERLAY_REQUIRED_ERROR = 'clip clip-1 text overlay text is required';
const AUTOSAVE_DEBOUNCE_MS = 500;

type Overlay = { text: string; position_y: number };
type Clip = { id: string; start_seconds: number; end_seconds: number; title: string; edit?: { text_overlays?: Overlay[] } };
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
};

async function stubStreamJob(page: Page, faceCropReviewed: boolean): Promise<StreamStub> {
  const puts: Plan[] = [];
  let putError: string | null = null;
  const plan = editPlan(faceCropReviewed);
  await page.route(`**/api/streams/${JOB_ID}`, (route) =>
    route.fulfill(
      json({
        id: JOB_ID,
        status: 'ready',
        title: 'Clutch en Mirage',
        source_url: 'https://www.twitch.tv/donk/clip/ClutchOnMirage',
        probe: { width: 1920, height: 1080, duration_seconds: 60 },
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
    return route.fulfill(json({ ...body, updated_at: new Date().toISOString() }));
  });
  await page.route(`**/api/streams/${JOB_ID}/renders/**`, (route) => route.fulfill(json({ error: 'no render' }, 404)));
  await page.route(`**/api/streams/${JOB_ID}/source`, (route) => route.fulfill(json({ error: 'no source' }, 404)));
  await page.route('**/api/songs', (route) => route.fulfill(json({ songs: [] })));
  return {
    puts,
    rejectPuts(error) {
      putError = error;
    },
  };
}

const stepTitle = (page: Page, name: string) => page.getByRole('heading', { level: 2, name });
const railStep = (page: Page, name: RegExp) => page.getByRole('navigation', { name: 'Pasos' }).getByRole('button', { name });
const autosaveStatus = (page: Page) => page.getByRole('navigation', { name: 'Pasos' }).getByRole('status');
const cta = (page: Page, name: string) => page.getByRole('button', { name, exact: true });
const briefCheckbox = (page: Page) => page.getByRole('checkbox', { name: 'Apruebo el brief antes de renderizar' });

test.describe('stream editor', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('opens on step 01 while the facecam crop is unconfirmed, and the CTA leads back there', async ({ page }) => {
    const stub = await stubStreamJob(page, false);
    await gotoStudio(page, `/streams/${JOB_ID}`);

    await expect(stepTitle(page, '01 · Layout y facecam')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Confirmar recorte de facecam' })).toBeVisible();
    await expect(cta(page, 'Confirma el recorte primero')).toBeEnabled();
    await expect(briefCheckbox(page)).toBeDisabled();
    await expect(page.getByText('Confirma el recorte de facecam en el paso 01 para poder aprobar')).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Pasos' }).getByRole('button', { name: /Revisar/ })).toHaveCount(0);

    // The crop is edited on the monitor while the timeline stays on screen.
    await expect(page.getByLabel('Mover región de recorte del facecam')).toBeVisible();
    await expect(page.getByRole('region', { name: 'Timeline de la fuente' })).toBeVisible();

    await railStep(page, /Cortes/).click();
    await expect(stepTitle(page, '03 · Cortes → Shorts')).toBeVisible();
    await expect(page.getByLabel('Mover región de recorte del facecam')).toHaveCount(0);
    await cta(page, 'Confirma el recorte primero').click();
    await expect(stepTitle(page, '01 · Layout y facecam')).toBeVisible();

    // Opening an unchanged plan must not rewrite the server artifact.
    await page.waitForTimeout(AUTOSAVE_DEBOUNCE_MS * 2);
    expect(stub.puts).toHaveLength(0);
  });

  test('confirming the crop unlocks the brief, approving it enables the CTA, and the brief reads as a clock', async ({ page }) => {
    const stub = await stubStreamJob(page, false);
    await gotoStudio(page, `/streams/${JOB_ID}`);

    await page.getByRole('button', { name: 'Confirmar recorte de facecam' }).click();
    await expect(page.getByRole('button', { name: 'Recorte confirmado' })).toBeVisible();
    await expect(railStep(page, /Layout/)).toContainText('recorte ✓');
    await expect(cta(page, 'Aprueba el brief')).toBeDisabled();

    await briefCheckbox(page).check();
    await expect(cta(page, 'Crear Shorts →')).toBeEnabled();
    await expect(autosaveStatus(page)).toHaveText('✓ Guardado · local + servidor');
    await expect.poll(() => stub.puts.at(-1)?.face_crop_reviewed).toBe(true);

    const briefLine = page.getByTitle(/^Facecam 40 — .*de salida aprox\./);
    await expect(briefLine).toContainText('1 clip · 0:12 de salida aprox.');
    await page.getByText('Brief creativo', { exact: true }).click();
    const clipsItem = page.getByRole('definition').filter({ hasText: 'de salida aprox.' });
    await expect(clipsItem).toHaveText('1 clip · 0:12 de salida aprox.');
    await expect(clipsItem).not.toHaveText(/-\d/);
    const summaryBox = await page.getByText('Brief creativo', { exact: true }).boundingBox();
    const checkboxBox = await briefCheckbox(page).boundingBox();
    expect(summaryBox).not.toBeNull();
    expect(checkboxBox).not.toBeNull();
    if (summaryBox !== null && checkboxBox !== null) {
      expect(Math.abs(checkboxBox.y - summaryBox.y)).toBeLessThan(12);
    }
  });

  test('"Añadir texto" keeps a blank overlay local until text is typed, and a cleared text leaves the plan', async ({ page }) => {
    const stub = await stubStreamJob(page, true);
    await gotoStudio(page, `/streams/${JOB_ID}`);

    await expect(stepTitle(page, '03 · Cortes → Shorts')).toBeVisible();
    await page.getByRole('button', { name: '+ Texto' }).click();
    await page.getByRole('button', { name: 'Añadir texto' }).click();
    const textField = page.getByLabel('Texto', { exact: true });
    await expect(textField).toBeVisible();
    await expect(textField).toHaveAttribute('aria-invalid', 'true');
    await page.waitForTimeout(AUTOSAVE_DEBOUNCE_MS * 2);
    expect(stub.puts).toHaveLength(0);

    await textField.fill('NICE SHOT');
    await expect.poll(() => stub.puts.at(-1)?.clips[0]?.edit?.text_overlays?.[0]?.text).toBe('NICE SHOT');
    await expect(autosaveStatus(page)).toHaveText('✓ Guardado · local + servidor');

    await textField.fill('');
    await expect.poll(() => stub.puts.length).toBeGreaterThan(1);
    expect(stub.puts.at(-1)?.clips[0]?.edit).toBeUndefined();
    await expect(textField).toBeVisible();
    await expect(autosaveStatus(page)).toHaveText('✓ Guardado · local + servidor');
  });

  test('a rejected autosave PUT shows the failed state, then clears once a save lands', async ({ page }) => {
    const stub = await stubStreamJob(page, true);
    stub.rejectPuts(OVERLAY_REQUIRED_ERROR);
    await gotoStudio(page, `/streams/${JOB_ID}`);

    await page.getByLabel('Título del corte 01').fill('Clutch 1v3 final');
    await expect(autosaveStatus(page)).toHaveText('Sin guardar en el servidor');
    await expect(page.getByRole('alert').filter({ hasText: OVERLAY_REQUIRED_ERROR })).toBeVisible();
    expect(stub.puts.length).toBeGreaterThan(0);

    stub.rejectPuts(null);
    await page.getByLabel('Título del corte 01').fill('Clutch 1v3 final ronda 30');
    await expect(autosaveStatus(page)).toHaveText('✓ Guardado · local + servidor');
    await expect(page.getByRole('alert').filter({ hasText: OVERLAY_REQUIRED_ERROR })).toHaveCount(0);
  });
});
