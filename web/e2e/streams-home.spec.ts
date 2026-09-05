import { expect, test, type Page } from '@playwright/test';
import type { StreamJob } from '../lib/api/streams.ts';
import { SERVICE_UNAVAILABLE_CODE } from '../lib/api/types.ts';
import { gotoStudio, VALIDATION_WIDTHS } from './contract.ts';

const JOB: StreamJob = {
  id: '5c1d7e2a-3b4f-4c6d-9e8f-0a1b2c3d4e5f',
  status: 'ready',
  title: '1V4',
  source_url: 'https://www.twitch.tv/zacketizorcs2/clip/Clutch',
  probe: { width: 1920, height: 1080, duration_seconds: 50 },
  clip_count: 4,
  created_at: '2026-09-05T10:00:00Z',
};

async function stubStreams(page: Page, initialJobs: StreamJob[] = [JOB]): Promise<{ deletes: string[] }> {
  let jobs = initialJobs;
  const deletes: string[] = [];
  await page.route('**/api/streams', (route) => route.fulfill({
    json: route.request().method() === 'POST' ? JOB : { jobs },
  }));
  await page.route('**/api/streams/*', (route) => {
    if (route.request().method() === 'DELETE') {
      deletes.push(route.request().url());
      jobs = [];
      return route.fulfill({ status: 204 });
    }
    return route.fulfill({ json: JOB });
  });
  await page.route('**/api/streams/*/source', (route) => route.fulfill({ status: 404 }));
  return { deletes };
}

test.describe('stream import and projects', () => {
  test.use({ viewport: { width: 1440, height: 1000 } });

  test('imports a trimmed URL and optional project name through the existing API', async ({ page }) => {
    await stubStreams(page, []);
    await gotoStudio(page, '/streams');
    await page.getByLabel('Enlace del vídeo de Twitch, YouTube o Kick').fill(`  ${JOB.source_url}  `);
    await page.getByLabel('Nombre del proyecto').fill('  Clutch final  ');
    const request = page.waitForRequest((req) => req.url().endsWith('/api/streams') && req.method() === 'POST');
    await page.getByRole('button', { name: 'Importar vídeo', exact: true }).click();
    expect((await request).postDataJSON()).toEqual({ source_url: JOB.source_url, title: 'Clutch final' });
    await expect(page).toHaveURL(`/streams/${JOB.id}`);
  });

  test('keeps URL errors beside the field and clears them when the user edits', async ({ page }) => {
    await stubStreams(page, []);
    await gotoStudio(page, '/streams');
    const field = page.getByLabel('Enlace del vídeo de Twitch, YouTube o Kick');
    await page.getByRole('button', { name: 'Importar vídeo', exact: true }).click();
    await expect(field).toHaveAttribute('aria-invalid', 'true');
    await expect(page.locator('#stream-url-error')).toContainText('Pega una URL');
    await field.fill('https://example.com/image.png');
    await expect(page.locator('#stream-url-error')).toHaveCount(0);
    await page.getByRole('button', { name: 'Importar vídeo', exact: true }).click();
    await expect(page.locator('#stream-url-error')).toContainText('no a un vídeo');
    await field.fill('https://www.youtube.com/watch?v=clip');
    await expect(field).not.toHaveAttribute('aria-invalid');
  });

  test('the file chooser sends an MP4 and the shared optional title', async ({ page }) => {
    await stubStreams(page);
    await gotoStudio(page, '/streams');
    await page.getByLabel('Nombre del proyecto').fill('Mi grabación');
    const chooser = page.waitForEvent('filechooser');
    await page.getByRole('button', { name: 'Seleccionar archivo MP4' }).click();
    const request = page.waitForRequest((req) => req.url().endsWith('/api/streams') && req.method() === 'POST');
    await (await chooser).setFiles({ name: 'clutch.mp4', mimeType: 'video/mp4', buffer: Buffer.from('test video') });
    const upload = await request;
    expect(upload.headers()['content-type']).toContain('multipart/form-data');
    expect(upload.postData()).toContain('filename="clutch.mp4"');
    expect(upload.postData()).toContain('Mi grabación');
    await expect(page).toHaveURL(`/streams/${JOB.id}`);
  });

  test('dropping a file imports it and disables both choices until the request finishes', async ({ page }) => {
    await stubStreams(page);
    let finishUpload: () => void = () => {};
    const uploadPending = new Promise<void>((resolve) => { finishUpload = resolve; });
    await page.route('**/api/streams', async (route) => {
      if (route.request().method() === 'POST') {
        await uploadPending;
        return route.fulfill({ status: 400, json: { error: 'Prueba con otro MP4.' } });
      }
      return route.fulfill({ json: { jobs: [JOB] } });
    });
    await gotoStudio(page, '/streams');
    const dropzone = page.getByRole('button', { name: 'Seleccionar archivo MP4' });
    const transfer = await page.evaluateHandle(() => {
      const data = new DataTransfer();
      data.items.add(new File(['test video'], 'drop.mp4', { type: 'video/mp4' }));
      return data;
    });
    try {
      await dropzone.dispatchEvent('dragover', { dataTransfer: transfer });
      await expect(dropzone).toContainText('Suelta tu vídeo aquí');
      await dropzone.dispatchEvent('drop', { dataTransfer: transfer });
      await expect(dropzone).toBeDisabled();
      await expect(page.getByRole('button', { name: 'Importando…' })).toBeDisabled();
      await expect(page.getByLabel('Nombre del proyecto')).toBeDisabled();
      await expect(page.locator('input[type="file"]')).toBeDisabled();
    } finally {
      finishUpload();
      await transfer.dispose();
    }
    await expect(page.getByRole('alert').filter({ hasText: 'Prueba con otro MP4.' })).toBeVisible();
    await expect(dropzone).toBeEnabled();
    await expect(page.getByRole('button', { name: 'Importar vídeo', exact: true })).toBeEnabled();
  });

  test('continues a draft and keeps deletion behind its separate confirmation', async ({ page }) => {
    const { deletes } = await stubStreams(page);
    await gotoStudio(page, '/streams');
    await expect(page.getByRole('region', { name: 'Tus proyectos' })).toContainText('4 cortes');
    await page.getByRole('button', { name: 'Continuar edición: 1V4' }).click();
    await expect(page).toHaveURL(`/streams/${JOB.id}`);
    await gotoStudio(page, '/streams');
    await page.getByRole('button', { name: 'Borrar 1V4', exact: true }).click();
    expect(deletes).toHaveLength(0);
    await expect(page).toHaveURL('/streams');
    await page.getByRole('button', { name: 'Confirmar borrar 1V4' }).click();
    await expect(page.getByText('Tus proyectos aparecerán aquí.', { exact: false })).toBeVisible();
    expect(deletes).toHaveLength(1);
  });

  test('list failures remain recoverable without hiding the import form', async ({ page }) => {
    await page.route('**/api/streams', (route) => route.fulfill({
      status: 503, json: { code: SERVICE_UNAVAILABLE_CODE },
    }));
    await gotoStudio(page, '/streams');
    await expect(page.getByRole('alert').filter({ hasText: 'offline' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Importar vídeo', exact: true })).toBeEnabled();
    await expect(page.getByText('Cargando streams')).toHaveCount(0);
    await page.route('**/api/streams', (route) => route.fulfill({ json: { jobs: [JOB] } }));
    await page.getByRole('button', { name: 'Reintentar' }).click();
    await expect(page.getByRole('button', { name: 'Continuar edición: 1V4' })).toBeVisible();
    await expect(page.getByRole('alert').filter({ hasText: 'offline' })).toHaveCount(0);
  });

  for (const width of VALIDATION_WIDTHS) {
    test(`project actions and import fit at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 1080 });
      await stubStreams(page);
      await gotoStudio(page, '/streams');
      const projects = page.getByRole('region', { name: 'Tus proyectos' });
      await expect(projects.getByRole('button', { name: 'Continuar edición: 1V4' })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Seleccionar archivo MP4' })).toBeVisible();
      const geometry = await page.evaluate(() => ({
        actual: document.documentElement.scrollWidth,
        viewport: document.documentElement.clientWidth,
      }));
      expect(geometry.actual).toBeLessThanOrEqual(geometry.viewport);
      await page.screenshot({ path: testInfo.outputPath(`streams-${width}.png`), fullPage: true });
    });
  }
});
