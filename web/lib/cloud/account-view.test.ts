import test from 'node:test';
import assert from 'node:assert/strict';
import {
  cloudAccessTag,
  cloudConnectionTag,
  cloudDeviceLinked,
  cloudLinkIdle,
  cloudLinkPhase,
  cloudQueueLine,
  cloudUnlinkWarning,
  cloudUsageLines,
  safeVerifyUrl,
} from './account-view.ts';
import type { CloudAccount, CloudAccountState, CloudLink } from './parse.ts';

const NOW = Date.parse('2026-10-09T10:00:00Z');

function unlinked(link: CloudLink | null, error: CloudAccount['error'] = null): CloudAccountState {
  return {
    kind: 'ready',
    account: {
      portalUrl: 'https://cliphub.rechedev.cloud',
      linked: false,
      link,
      user: null,
      access: null,
      limits: null,
      usage: null,
      kinds: [],
      queue: null,
      needsApproval: false,
      error,
    },
  };
}

function linked(overrides: Partial<CloudAccount> = {}): CloudAccount {
  return {
    portalUrl: 'https://cliphub.rechedev.cloud',
    linked: true,
    link: null,
    user: { name: 'Luis', email: 'luis@example.com' },
    access: 'allowed',
    limits: { maxActive: 3, dailySeconds: 5400 },
    usage: { active: 1, secondsLast24h: 1200, secondsCommitted: 480 },
    kinds: ['short'],
    queue: null,
    needsApproval: false,
    error: null,
    ...overrides,
  };
}

const PENDING: CloudLink = {
  status: 'pending',
  userCode: 'K7QM-2XHD',
  verifyUrl: 'https://cliphub.rechedev.cloud/link?code=K7QM-2XHD',
  expiresAt: NOW + 600_000,
};

test('a pending link shows its code and the page to open', () => {
  assert.deepEqual(cloudLinkPhase({ account: unlinked(PENDING), startError: null, now: NOW }), {
    kind: 'pending',
    userCode: 'K7QM-2XHD',
    verifyUrl: 'https://cliphub.rechedev.cloud/link?code=K7QM-2XHD',
  });
});

test('the dialog flips to linked as soon as the account is, whatever happened before', () => {
  const phase = cloudLinkPhase({ account: { kind: 'ready', account: linked() }, startError: { status: 429 }, now: NOW });
  assert.deepEqual(phase, { kind: 'linked', name: 'Luis' });
});

test('a code past its expiry reads as expired even before the orchestrator says so', () => {
  assert.equal(cloudLinkPhase({ account: unlinked(PENDING), startError: null, now: NOW + 600_000 }).kind, 'expired');
  assert.equal(cloudLinkPhase({ account: unlinked({ ...PENDING, status: 'expired' }), startError: null, now: NOW }).kind, 'expired');
  assert.equal(cloudLinkPhase({ account: unlinked({ ...PENDING, status: 'denied' }), startError: null, now: NOW }).kind, 'denied');
});

test('each way the link can fail to start has its own sentence', () => {
  const messages = [
    cloudLinkPhase({ account: unlinked(null), startError: { status: 429 }, now: NOW }),
    cloudLinkPhase({ account: unlinked(null), startError: { code: 'portal_unreachable', status: 503 }, now: NOW }),
    cloudLinkPhase({ account: { kind: 'offline' }, startError: null, now: NOW }),
    cloudLinkPhase({ account: { kind: 'unavailable' }, startError: null, now: NOW }),
    cloudLinkPhase({ account: unlinked(null), startError: { status: 500 }, now: NOW }),
  ].map((phase) => (phase.kind === 'failed' ? phase.message : phase.kind));
  assert.equal(new Set(messages).size, 5);
  assert.match(messages[0], /Demasiados intentos/);
  assert.match(messages[1], /No se pudo conectar con la nube/);
});

test('an unreachable portal with no link is a failure, not an endless spinner', () => {
  assert.equal(cloudLinkPhase({ account: unlinked(null, 'portal_unreachable'), startError: null, now: NOW }).kind, 'failed');
  assert.equal(cloudLinkPhase({ account: unlinked(null), startError: null, now: NOW }).kind, 'starting');
});

test('only an https page or a loopback development portal may be opened', () => {
  assert.equal(safeVerifyUrl('https://cliphub.rechedev.cloud/link?code=K7QM-2XHD'), 'https://cliphub.rechedev.cloud/link?code=K7QM-2XHD');
  assert.equal(safeVerifyUrl('http://127.0.0.1:3000/link?code=A'), 'http://127.0.0.1:3000/link?code=A');
  assert.equal(safeVerifyUrl('http://localhost:3000/link'), 'http://localhost:3000/link');
  for (const bad of ['http://cliphub.rechedev.cloud/link', 'javascript:alert(1)', 'file:///C:/x', 'https://user:pw@evil.test/', 'not a url', '']) {
    assert.equal(safeVerifyUrl(bad), null, bad);
  }
  const phase = cloudLinkPhase({ account: unlinked({ ...PENDING, verifyUrl: 'javascript:alert(1)' }), startError: null, now: NOW });
  assert.deepEqual(phase, { kind: 'pending', userCode: 'K7QM-2XHD', verifyUrl: null });
});

test('the access tag names each access state', () => {
  assert.deepEqual(cloudAccessTag(linked()), { text: 'Acceso activo', tone: 'success' });
  assert.equal(cloudAccessTag(linked({ access: 'pending' })).text, 'Pendiente de aprobación');
  assert.equal(cloudAccessTag(linked({ access: 'blocked' })).tone, 'danger');
  assert.equal(cloudAccessTag(linked({ error: 'portal_unreachable' })).text, 'Sin conexión con la nube');
});

test('usage is shown in minutes against the limit', () => {
  assert.deepEqual(cloudUsageLines(linked()), [
    'Tiempo de nube en las últimas 24 h: 20 de 90 min',
    'Reservado por tus vídeos en cola: 8 min',
    'Vídeos en la nube ahora: 1 de 3',
  ]);
  assert.deepEqual(cloudUsageLines(linked({ usage: { active: 0, secondsLast24h: 0, secondsCommitted: 0 } })), [
    'Tiempo de nube en las últimas 24 h: 0 de 90 min',
    'Vídeos en la nube ahora: 0 de 3',
  ]);
  assert.deepEqual(cloudUsageLines(linked({ usage: null })), []);
});

test('a PC counts as unlinked only once the account is known to be missing', () => {
  assert.equal(cloudDeviceLinked(unlinked(null)), false);
  assert.equal(cloudDeviceLinked(unlinked(null, 'unauthorized')), false);
  assert.equal(cloudDeviceLinked({ kind: 'ready', account: linked() }), true);
  // Unknown is not unlinked: a job keeps its last status while the account loads.
  assert.equal(cloudDeviceLinked({ kind: 'loading' }), true);
  assert.equal(cloudDeviceLinked({ kind: 'offline' }), true);
});

test('unlinking warns only while videos are in flight, and says what happens to them', () => {
  assert.equal(cloudUnlinkWarning(linked({ usage: { active: 0, secondsLast24h: 0, secondsCommitted: 0 } })), null);
  assert.equal(cloudUnlinkWarning(linked({ usage: null })), null);
  const one = cloudUnlinkWarning(linked());
  assert.match(one ?? '', /^Tienes 1 vídeo en marcha en la nube\. Seguirá allí/);
  const two = cloudUnlinkWarning(linked({ usage: { active: 2, secondsLast24h: 0, secondsCommitted: 0 } }));
  assert.match(two ?? '', /^Tienes 2 vídeos en marcha en la nube\. Seguirán allí/);
  assert.match(two ?? '', /hasta que conectes la misma cuenta otra vez/);
});

test('the connection tag says whether this PC is connected', () => {
  assert.equal(cloudConnectionTag({ kind: 'loading' }), null);
  assert.deepEqual(cloudConnectionTag({ kind: 'unavailable' }), { text: 'No disponible', tone: 'neutral' });
  assert.deepEqual(cloudConnectionTag(unlinked(null)), { text: 'Sin conectar', tone: 'neutral' });
  assert.deepEqual(cloudConnectionTag(unlinked(PENDING)), { text: 'Sin conectar', tone: 'neutral' });
  assert.deepEqual(cloudConnectionTag(unlinked(null, 'portal_unreachable')), { text: 'Sin conexión', tone: 'warning' });
  assert.deepEqual(cloudConnectionTag({ kind: 'ready', account: linked() }), { text: 'Conectada', tone: 'success' });
  assert.deepEqual(cloudConnectionTag({ kind: 'ready', account: linked({ access: 'pending' }) }), {
    text: 'Pendiente de aprobación',
    tone: 'warning',
  });
});

test('the cloud page asks for a code only after the user clicks', () => {
  assert.equal(cloudLinkIdle(unlinked(null)), true);
  assert.equal(cloudLinkIdle(unlinked(null, 'unauthorized')), true);
  assert.equal(cloudLinkIdle(unlinked(null, 'portal_unreachable')), false);
  assert.equal(cloudLinkIdle(unlinked(PENDING)), false);
  assert.equal(cloudLinkIdle({ kind: 'ready', account: linked() }), false);
  assert.equal(cloudLinkIdle({ kind: 'loading' }), false);
});

test('the queue line says how busy the cloud is', () => {
  assert.equal(cloudQueueLine(linked()), null);
  assert.equal(cloudQueueLine(linked({ queue: { state: 'online', queued: 0, waitSeconds: {} } })), 'Cola de la nube: sin vídeos en espera.');
  assert.equal(cloudQueueLine(linked({ queue: { state: 'online', queued: 1, waitSeconds: {} } })), 'Cola de la nube: 1 vídeo en espera.');
  assert.match(cloudQueueLine(linked({ queue: { state: 'paused', queued: 4, waitSeconds: {} } })) ?? '', /en pausa/);
  assert.match(cloudQueueLine(linked({ queue: { state: 'offline', queued: 0, waitSeconds: {} } })) ?? '', /no está conectado/);
});
