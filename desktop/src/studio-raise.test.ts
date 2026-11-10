import test from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import {
  handleOpenRequest,
  isStudioPagePath,
  OPEN_REQUEST_FILE,
  type OpenRequestOptions,
  type RaisableWindow,
} from './studio-raise.ts';

const WEB = 'http://127.0.0.1:4200';
const ID = 'a'.repeat(32);

function fakeWindow(options: { url: string; fails?: boolean }): RaisableWindow & { calls: string[] } {
  const calls: string[] = [];
  return {
    calls,
    isMinimized: () => true,
    restore: () => calls.push('restore'),
    show: () => calls.push('show'),
    focus: () => calls.push('focus'),
    loadURL: async (url: string) => {
      calls.push(`load ${url}`);
      if (options.fails) throw new Error('ERR_FAILED');
    },
    webContents: { getURL: () => options.url, send: () => calls.push('send') },
  };
}

function setup(win: RaisableWindow, extra: Partial<OpenRequestOptions> = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'studio-raise-'));
  const options: OpenRequestOptions = {
    userData: dir, window: () => win, webOrigin: () => WEB, log: () => {}, routeInApp: async () => false, ...extra,
  };
  const request = (body: string): void => fs.writeFileSync(path.join(dir, OPEN_REQUEST_FILE), body);
  const ack = (): unknown => JSON.parse(fs.readFileSync(path.join(dir, `open-ack-${ID}.json`), 'utf8'));
  return { dir, options, request, ack };
}

test('only app pages pass', () => {
  for (const ok of ['/replay?r=a:3#p', '/informes/x', '/equipos/x?tab=preparar']) assert.equal(isStudioPagePath(ok), true, ok);
  for (const bad of ['/api/library/demos', '/_next/static/a.js', '/hud/a.png', '//evil.example', 'replay', '/a\\b', `/${'x'.repeat(17000)}`]) {
    assert.equal(isStudioPagePath(bad), false, bad);
  }
});

test('a window on Studio routes in the app and acks once the app took it', async () => {
  const win = fakeWindow({ url: `${WEB}/informes` });
  const routed: string[] = [];
  const { options, request, ack } = setup(win, { routeInApp: async (r) => routed.push(r.path) > 0 });
  request(JSON.stringify({ version: 2, id: ID, path: '/replay?r=a:3' }));
  await handleOpenRequest(options);
  assert.deepEqual(routed, ['/replay?r=a:3']);
  assert.ok(!win.calls.some((c) => c.startsWith('load')), 'no full reload');
  assert.deepEqual(ack(), { version: 2, id: ID, ok: true, reason: '' });
});

test('a failed load, a refused path and a loading window ack false', async () => {
  const failing = setup(fakeWindow({ url: 'about:blank', fails: true }));
  failing.request(JSON.stringify({ version: 2, id: ID, path: '/replay' }));
  await handleOpenRequest(failing.options);
  assert.equal((failing.ack() as { ok: boolean }).ok, false);

  const refused = setup(fakeWindow({ url: `${WEB}/` }));
  refused.request(JSON.stringify({ version: 2, id: ID, path: '/api/library/reports' }));
  await handleOpenRequest(refused.options);
  assert.deepEqual(refused.ack(), { version: 2, id: ID, ok: false, reason: 'not a Studio page path' });

  const loading = setup(fakeWindow({ url: `${WEB}/` }), { webOrigin: () => null });
  loading.request(JSON.stringify({ version: 2, id: ID, path: '/replay' }));
  await handleOpenRequest(loading.options);
  assert.equal((loading.ack() as { ok: boolean }).ok, false);
});

test('an oversized or non-file request is dropped unread', async () => {
  const big = setup(fakeWindow({ url: `${WEB}/` }));
  big.request(JSON.stringify({ version: 2, id: ID, path: `/${'x'.repeat(70 * 1024)}` }));
  await handleOpenRequest(big.options);
  assert.equal(fs.existsSync(path.join(big.dir, OPEN_REQUEST_FILE)), false);
  assert.equal(fs.existsSync(path.join(big.dir, `open-ack-${ID}.json`)), false);

  const dir = setup(fakeWindow({ url: `${WEB}/` }));
  fs.mkdirSync(path.join(dir.dir, OPEN_REQUEST_FILE));
  await handleOpenRequest(dir.options);
  assert.equal(fs.existsSync(path.join(dir.dir, `open-ack-${ID}.json`)), false);
});
