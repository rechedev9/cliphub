import { expect, test } from '@playwright/test';

const DEVICE_ID = '11111111-1111-4111-8111-111111111111';
const DEVICE_SECRET = 'b'.repeat(64);
const BROWSER_CAPABILITY = 'a'.repeat(64);

test('account, pairing, local checks, upload, and ranged media stay in the intended planes', async ({ page, request }) => {
  const pairingResponse = await request.post('/api/agent/pairings', {
    data: {
      deviceId: DEVICE_ID,
      name: 'E2E Gaming PC',
      platform: 'windows-amd64',
      version: '2.5.0',
      secret: DEVICE_SECRET,
    },
  });
  expect(pairingResponse.status()).toBe(201);
  const pairingBody = await pairingResponse.json() as { pairing: { code: string } };

  await page.goto(`/connect?pair=${pairingBody.pairing.code}#agent=43123.${BROWSER_CAPABILITY}`);
  await expect(page.getByRole('heading', { name: 'Conecta el motor local' })).toBeVisible();
  await page.getByRole('link', { name: 'INICIAR SESIÓN' }).click();
  await page.getByRole('link', { name: 'Crear cuenta' }).click();
  await expect(page.getByRole('heading', { name: 'Crea tu espacio local' })).toBeVisible();
  await page.getByLabel('Correo').fill('e2e@example.com');
  await page.getByLabel('Contraseña').fill('a-secure-e2e-password');
  await page.getByRole('button', { name: 'CREAR CUENTA' }).click();

  await expect(page.getByText('Conexión local autenticada.')).toBeVisible();
  await expect(page.getByText('CS2, HLAE y FFmpeg preparados.')).toBeVisible();
  await expect(page.getByRole('button', { name: 'ABRIR STUDIO' })).toBeVisible();

  const localUpload = await page.evaluate(async () => {
    const form = new FormData();
    form.append('demo', new File(['demo-bytes'], 'match.dem'));
    const response = await fetch('http://127.0.0.1:43123/api/demos/scan', {
      method: 'POST',
      headers: { Authorization: `Bearer ${'a'.repeat(64)}` },
      body: form,
      credentials: 'omit',
      mode: 'cors',
    });
    return { status: response.status, body: await response.json() as unknown };
  });
  expect(localUpload).toEqual({ status: 201, body: { jobId: 'local-e2e-job' } });

  const media = await page.evaluate(async () => {
    const response = await fetch('/api/demos/11111111-1111-4111-8111-111111111111/renders/e2e/videos/e2e.mp4', {
      headers: { Range: 'bytes=0-10' },
    });
    return {
      status: response.status,
      range: response.headers.get('content-range'),
      body: await response.text(),
    };
  });
  expect(media).toEqual({ status: 206, range: 'bytes 0-10/11', body: 'local-media' });

  const heartbeat = await request.post('/api/agent/heartbeat', {
    headers: { Authorization: `Bearer ${DEVICE_SECRET}` },
    data: { deviceId: DEVICE_ID, version: '2.5.0' },
  });
  expect(heartbeat.status()).toBe(204);
  const devices = await page.evaluate(async () => {
    const response = await fetch('/api/account/devices');
    return response.json() as Promise<{ devices: Array<{ id: string; online: boolean }> }>;
  });
  expect(devices.devices).toEqual([expect.objectContaining({ id: DEVICE_ID, online: true })]);

  await page.getByRole('button', { name: 'ABRIR STUDIO' }).click();
  await expect(page.getByRole('heading', { name: 'EMPIEZA AQUÍ' })).toBeVisible();
});
