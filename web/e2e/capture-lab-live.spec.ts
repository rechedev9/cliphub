import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { expect, test } from '@playwright/test';
import { gotoStudio } from './contract.ts';

const enabled = process.env.CAPTURE_LAB_LIVE === '1';
const jobId = process.env.CAPTURE_LAB_JOB_ID ?? '';
const variant = process.env.CAPTURE_LAB_VARIANT ?? 'viral-60-clean';
const expectedVideo = process.env.CAPTURE_LAB_EXPECTED_VIDEO ?? '';

test.describe('Capture Lab live Studio boundary', () => {
  test.skip(!enabled, 'requires the explicit Capture Lab live orchestrator');

  test('shows and downloads the synthetic render through real same-origin proxies', async ({ page }, testInfo) => {
    expect(jobId).toMatch(/^[0-9a-f-]{36}$/);
    expect(expectedVideo).not.toBe('');
    await page.addInitScript(({ id, renderVariant }) => {
      window.localStorage.setItem('cliphub.reels.v1', JSON.stringify([{
        videoId: `${id}__capturelab`,
        jobId: id,
        segmentIds: ['seg-001'],
        mode: 'clean',
        variant: renderVariant,
        editConfig: {
          format: 'short-9x16', killEffect: 'clean', transition: 'cut',
          intro: false, outro: false, hookText: false, killCounter: false,
          matchRecap: false, voiceComms: false, nativeHud: false,
          coverStrategy: 'no-cover', introText: '', outroText: '',
        },
        title: 'Capture Lab · synthetic verified render',
        map: 'de_nuke', targetName: 'Joey-', createdAt: 1,
      }]));
    }, { id: jobId, renderVariant: variant });

    await gotoStudio(page, '/videos');
    const apiContract = await page.evaluate(async ({ id, renderVariant }) => {
      const read = async (path: string) => {
        const response = await fetch(path);
        return { status: response.status, body: await response.text() };
      };
      return {
        job: await read(`/api/demos/${id}/status`),
        render: await read(`/api/demos/${id}/renders/${renderVariant}`),
      };
    }, { id: jobId, renderVariant: variant });
    expect(apiContract.job.status, apiContract.job.body).toBe(200);
    expect(apiContract.render.status, apiContract.render.body).toBe(200);
    expect(JSON.parse(apiContract.render.body).status).toBe('ready');
    await expect(page.getByTitle('Capture Lab · synthetic verified render', { exact: true })).toBeVisible();
    const mp4 = page.getByRole('button', { name: 'MP4' });
    await expect(mp4).toBeVisible();
    await expect(mp4).toBeEnabled();

    const downloadPromise = page.waitForEvent('download');
    await mp4.click();
    const download = await downloadPromise;
    const downloadedPath = await download.path();
    expect(downloadedPath).not.toBeNull();
    const [actual, expected] = await Promise.all([
      readFile(downloadedPath!),
      readFile(expectedVideo),
    ]);
    const digest = (value: Buffer) => createHash('sha256').update(value).digest('hex');
    expect(digest(actual)).toBe(digest(expected));
    await page.screenshot({ path: testInfo.outputPath('capture-lab-library-ready.png'), fullPage: true });
  });
});
