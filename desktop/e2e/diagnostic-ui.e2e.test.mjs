import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { createE2EProfile } from '../scripts/e2e-profile.mjs';
import { E2E_BOOT_DEADLINE_MS } from '../scripts/e2e-boot-budget.mjs';
const require = createRequire(import.meta.url);
const { _electron } = require('playwright-core');
const desktop = join(dirname(fileURLToPath(import.meta.url)),'..');

test('diagnostic settings, renderer errors, receipts and revocation in the real Studio UI', {timeout: E2E_BOOT_DEADLINE_MS+90_000}, async () => {
  const profile = createE2EProfile('diagnostic-ui');
  let app;
  const artifacts = join(desktop,'e2e','artifacts','diagnostics');
  mkdirSync(artifacts,{recursive:true});
  try {
    app = await _electron.launch({executablePath:require('electron'),args:[join(desktop,'e2e','diagnostic-ui-bootstrap.cjs')],cwd:desktop,env:profile.environment()});
    const page = await app.firstWindow();
    await page.waitForURL(/^http:\/\/127\.0\.0\.1:\d+\/clips/, {timeout:E2E_BOOT_DEADLINE_MS});
    await page.getByRole('button',{name:'Mantener activado',exact:true}).click();
    await page.goto(new URL('/settings',page.url()).href);
    const panel = page.getByRole('region',{name:'Diagnósticos'});
    await panel.waitFor();
    // Toggling through settings guarantees a persisted and acknowledged choice.
    if (await panel.getByRole('button',{name:'DESACTIVAR DIAGNÓSTICOS'}).count()) await panel.getByRole('button',{name:'DESACTIVAR DIAGNÓSTICOS'}).click();
    await panel.getByRole('button',{name:'ACTIVAR DIAGNÓSTICOS'}).click();
    await panel.getByRole('button',{name:'DESACTIVAR DIAGNÓSTICOS'}).waitFor();
    await page.evaluate(()=>window.dispatchEvent(new ErrorEvent('error',{message:'UI_CANARY_CAUSE: renderer diagnostic',error:new Error('UI_CANARY_CAUSE: renderer diagnostic token=private-ui-token')})));
    await page.evaluate(()=>window.dispatchEvent(new PromiseRejectionEvent('unhandledrejection',{promise:Promise.resolve(),reason:new Error('UI_REJECTION_CAUSE: asynchronous operation')})));
    const receiptFile = join(profile.root,'diagnostic-ui-receipts.jsonl');
    const deadline = Date.now()+20_000;
    let received = false;
    while (Date.now()<deadline) {
      try { const text = readFileSync(receiptFile,'utf8'); received = text.includes('UI_CANARY_CAUSE') && text.includes('UI_REJECTION_CAUSE'); } catch { /* No receipt yet. */ }
      if (received) break;
      await new Promise((resolve)=>setTimeout(resolve,200));
    }
    assert.equal(received,true,'renderer cause did not reach the acknowledged transport');
    await page.reload();
    await panel.getByText('Última recepción confirmada').waitFor();
    const receipts = readFileSync(receiptFile,'utf8');
    assert.match(receipts,/UI_CANARY_CAUSE/);
    assert.doesNotMatch(receipts,/private-ui-token/);
    for (const [name,width,height] of [['mobile',390,844],['laptop',1366,768],['desktop',1920,1080]]) {
      await page.setViewportSize({width,height});
      await panel.scrollIntoViewIfNeeded();
      assert.equal(await panel.evaluate((node)=>node.scrollWidth<=node.clientWidth),true,`${name} overflow`);
      const button = panel.getByRole('button',{name:'DESACTIVAR DIAGNÓSTICOS'});
      assert.equal(await button.isVisible(),true);
      await panel.screenshot({path:join(artifacts,`${name}.png`)});
    }
    await panel.getByRole('button',{name:'DESACTIVAR DIAGNÓSTICOS'}).click();
    await panel.getByRole('button',{name:'ACTIVAR DIAGNÓSTICOS'}).waitFor();
    const status = await page.evaluate(()=>window.cliphubSettings.getTelemetry());
    assert.equal(status.enabled,false);
    assert.equal(status.logDelivery.pendingBytes,0);
  } catch (error) {
    if (app) {
      const current = await app.firstWindow();
      await current.screenshot({path:join(artifacts,'failure.png')});
      const state = await current.evaluate(()=>window.cliphubSettings?.getTelemetry());
      writeFileSync(join(artifacts,'failure.json'),JSON.stringify({profile:profile.root,state,userData:await app.evaluate(({app})=>app.getPath('userData'))},null,2));
    }
    throw error;
  } finally {
    await app?.close();
    profile.dispose();
  }
});
