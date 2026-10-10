import test from 'node:test';
import assert from 'node:assert/strict';
import {
  CAPTURE_TARGET_STORAGE_KEY,
  captureTargetParam,
  cloudCoversLocalGap,
  loadCaptureTarget,
  produceShortHref,
  resolveCaptureTarget,
  saveCaptureTarget,
} from './capture-target.ts';
import type { CloudAccount, CloudAccountState } from './parse.ts';

const MATCH_ID = '22222222-2222-4222-8222-222222222222';

function account(overrides: Partial<CloudAccount> = {}): CloudAccountState {
  return {
    kind: 'ready',
    account: {
      portalUrl: 'https://cliphub.gravityroom.app',
      linked: true,
      link: null,
      user: { name: 'Luis', email: 'luis@example.com' },
      access: 'allowed',
      limits: { maxActive: 3, dailySeconds: 5400 },
      usage: { active: 1, secondsLast24h: 1200, secondsCommitted: 480 },
      kinds: ['short'],
      queue: { state: 'online', queued: 3, waitSeconds: { short: 1200 } },
      error: null,
      ...overrides,
    },
  };
}

const UNLINKED: Partial<CloudAccount> = { linked: false, user: null, access: null, limits: null, usage: null, kinds: [], queue: null };

function cloud(state: CloudAccountState) {
  return resolveCaptureTarget({ picked: 'cloud', remembered: null, localStatus: 'ready', account: state });
}

test('a PC that can record starts on this PC and creation is never gated', () => {
  for (const state of [account(), { kind: 'loading' }, { kind: 'unavailable' }, { kind: 'offline' }] as const) {
    const view = resolveCaptureTarget({ remembered: null, localStatus: 'ready', account: state });
    assert.equal(view.target, 'local', state.kind);
    assert.equal(view.canSubmit, true, state.kind);
    assert.equal(view.offerLink, false, state.kind);
    assert.equal(view.status.text, 'CS2 + HLAE listos en este PC', state.kind);
  }
});

test('this PC stays creatable in every local state, exactly as before the control existed', () => {
  for (const localStatus of [null, 'ready', 'warning', 'unconfigured', 'offline'] as const) {
    const view = resolveCaptureTarget({ picked: 'local', remembered: null, localStatus, account: account() });
    assert.equal(view.target, 'local');
    assert.equal(view.canSubmit, true, String(localStatus));
  }
});

test('the busy-CS2 line belongs to this PC only', () => {
  const local = resolveCaptureTarget({ remembered: null, localStatus: 'ready', localBusy: true, account: account() });
  assert.match(local.status.text, /CS2 ocupado/);
  const remote = resolveCaptureTarget({ picked: 'cloud', remembered: null, localStatus: 'ready', localBusy: true, account: account() });
  assert.doesNotMatch(remote.status.text, /CS2 ocupado/);
});

test('the last choice is remembered, and a click on this screen wins over it', () => {
  assert.equal(resolveCaptureTarget({ remembered: 'cloud', localStatus: 'ready', account: account() }).target, 'cloud');
  assert.equal(resolveCaptureTarget({ picked: 'local', remembered: 'cloud', localStatus: 'ready', account: account() }).target, 'local');
});

test('a PC without CS2 or HLAE starts on the cloud and says what is missing when asked', () => {
  const preselected = resolveCaptureTarget({ remembered: 'local', localStatus: 'unconfigured', account: account() });
  assert.equal(preselected.target, 'cloud');
  const local = resolveCaptureTarget({ picked: 'local', remembered: null, localStatus: 'unconfigured', account: account() });
  assert.match(local.status.text, /le falta CS2 o HLAE/);
  assert.match(local.status.text, /graba en la nube/);
});

test('a Studio without a cloud client never selects the cloud', () => {
  const view = resolveCaptureTarget({ picked: 'cloud', remembered: 'cloud', localStatus: 'unconfigured', account: { kind: 'unavailable' } });
  assert.equal(view.target, 'local');
  assert.equal(view.cloudSelectable, false);
  assert.equal(view.canSubmit, true);
  assert.doesNotMatch(view.status.text, /nube/);
});

test('the cloud line reports queue, wait and remaining minutes when ready', () => {
  const view = cloud(account());
  assert.equal(view.canSubmit, true);
  assert.equal(view.status.text, '3 en cola · empieza en unos 20 a 30 min · te quedan 62 min hoy');
});

test('an empty queue reads as no queue', () => {
  const view = cloud(account({ queue: { state: 'online', queued: 0, waitSeconds: { short: 0 } } }));
  assert.equal(view.status.text, 'Sin cola · empieza en menos de 5 min · te quedan 62 min hoy');
});

test('not linked asks for the account and offers the link button', () => {
  const view = cloud(account(UNLINKED));
  assert.equal(view.status.text, 'Conecta tu cuenta para grabar en la nube');
  assert.equal(view.offerLink, true);
  assert.equal(view.canSubmit, false);
});

test('a link in progress shows the code to confirm', () => {
  const view = cloud(
    account({ ...UNLINKED, link: { status: 'pending', userCode: 'K7QM-2XHD', verifyUrl: 'https://cliphub.gravityroom.app/link', expiresAt: null } }),
  );
  assert.match(view.status.text, /K7QM-2XHD/);
  assert.equal(view.offerLink, true);
});

test('a device revoked in the portal returns to the link button', () => {
  const view = cloud(account({ ...UNLINKED, error: 'unauthorized' }));
  assert.match(view.status.text, /ya no está conectado/);
  assert.equal(view.offerLink, true);
  assert.equal(view.canSubmit, false);
});

test('an unreachable portal blocks the cloud and points at this PC, linked or not', () => {
  for (const state of [account({ error: 'portal_unreachable' }), account({ ...UNLINKED, error: 'portal_unreachable' })]) {
    const view = cloud(state);
    assert.equal(view.status.text, 'No se puede conectar con la nube ahora. Puedes grabar en este PC');
    assert.equal(view.canSubmit, false);
    assert.equal(view.offerLink, false);
  }
});

test('access states each have their own line and block the button', () => {
  // The line is all the producer shows before a submit: it has to say there is still a way to record.
  assert.equal(cloud(account({ access: 'pending' })).status.text, 'Tu cuenta está pendiente de aprobación. Hasta entonces puedes grabar en este PC');
  assert.equal(cloud(account({ access: 'blocked' })).status.text, 'Tu cuenta no tiene acceso a la nube');
  for (const access of ['pending', 'blocked', null] as const) {
    assert.equal(cloud(account({ access })).canSubmit, false, String(access));
  }
});

test('limits block the button with the specific reason', () => {
  const active = cloud(account({ usage: { active: 3, secondsLast24h: 0, secondsCommitted: 0 } }));
  assert.equal(active.canSubmit, false);
  assert.match(active.status.text, /Ya tienes 3 vídeos en la nube/);

  const daily = cloud(account({ usage: { active: 0, secondsLast24h: 5000, secondsCommitted: 400 } }));
  assert.equal(daily.canSubmit, false);
  assert.match(daily.status.text, /tiempo de nube de hoy/);

  const kind = cloud(account({ kinds: [] }));
  assert.equal(kind.canSubmit, false);
  assert.match(kind.status.text, /todavía no admite Shorts/);
});

test('a paused or offline cloud still accepts jobs and says so without a time', () => {
  const paused = cloud(account({ queue: { state: 'paused', queued: 5, waitSeconds: { short: null } } }));
  assert.equal(paused.status.text, 'La nube está en pausa. Puedes ponerte en cola o grabar en este PC');
  assert.equal(paused.canSubmit, true);
  const offline = cloud(account({ queue: { state: 'offline', queued: 5, waitSeconds: { short: null } } }));
  assert.equal(offline.canSubmit, true);
  assert.doesNotMatch(offline.status.text, /empieza en/);
});

test('while the account loads or the local service is down the cloud cannot be submitted', () => {
  assert.equal(cloud({ kind: 'loading' }).canSubmit, false);
  const offline = cloud({ kind: 'offline' });
  assert.equal(offline.canSubmit, false);
  assert.match(offline.status.text, /servicio local/);
});

test('the choice persists under its key and survives broken storage', () => {
  const store = new Map<string, string>();
  const storage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => void store.set(key, value),
  };
  assert.equal(loadCaptureTarget(storage), null);
  saveCaptureTarget(storage, 'cloud');
  assert.equal(store.get(CAPTURE_TARGET_STORAGE_KEY), 'cloud');
  assert.equal(loadCaptureTarget(storage), 'cloud');
  store.set(CAPTURE_TARGET_STORAGE_KEY, 'moon');
  assert.equal(loadCaptureTarget(storage), null);

  const broken = {
    getItem: (): string => {
      throw new Error('denied');
    },
    setItem: (): void => {
      throw new Error('quota');
    },
  };
  assert.equal(loadCaptureTarget(broken), null);
  assert.doesNotThrow(() => saveCaptureTarget(broken, 'local'));
  assert.equal(loadCaptureTarget(null), null);
});

test('the hub links back to the Short producer with the target preselected', () => {
  const href = produceShortHref(MATCH_ID, 'cloud');
  const url = new URL(href, 'http://127.0.0.1');
  assert.equal(url.pathname, `/clips/${MATCH_ID}/nuevo`);
  assert.equal(url.searchParams.get('formato'), 'short');
  assert.equal(captureTargetParam(url.searchParams.get('destino')), 'cloud');
  assert.equal(captureTargetParam(new URL(produceShortHref(MATCH_ID, 'local'), 'http://127.0.0.1').searchParams.get('destino')), 'local');
  assert.equal(captureTargetParam('luna'), null);
  assert.equal(captureTargetParam(['nube', 'pc']), null);
  assert.equal(captureTargetParam(undefined), null);
});

test('the sidebar suggests the cloud only for a PC that cannot record and an allowed account', () => {
  assert.equal(cloudCoversLocalGap('unconfigured', account()), true);
  assert.equal(cloudCoversLocalGap('ready', account()), false);
  assert.equal(cloudCoversLocalGap('unconfigured', account({ access: 'pending' })), false);
  assert.equal(cloudCoversLocalGap('unconfigured', account(UNLINKED)), false);
  assert.equal(cloudCoversLocalGap('unconfigured', account({ error: 'portal_unreachable' })), false);
  assert.equal(cloudCoversLocalGap('unconfigured', { kind: 'unavailable' }), false);
  assert.equal(cloudCoversLocalGap(null, account()), false);
});
