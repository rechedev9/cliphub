import assert from 'node:assert/strict';
import test from 'node:test';
import { createCalloutNameStore, type CalloutNameLoader } from './callouts.ts';

function counting(loader: CalloutNameLoader): { loader: CalloutNameLoader; calls: string[] } {
  const calls: string[] = [];
  return {
    calls,
    loader: (map) => {
      calls.push(map);
      return loader(map);
    },
  };
}

test('a place reads as CS2 says it until the map loads, then by the team name', async () => {
  const { loader, calls } = counting(async () => [{ token: 'TopofMid', name: 'Medio' }, { token: 'BombsiteA' }]);
  const store = createCalloutNameStore(loader);
  assert.equal(store.label()('de_mirage', 'TopofMid'), 'Top of Mid');
  assert.equal(store.label()('de_mirage', ''), '');
  await Promise.all([store.load('de_mirage'), store.load('de_mirage')]);
  await store.load('de_mirage');
  assert.deepEqual(calls, ['de_mirage']);
  const label = store.label();
  assert.equal(label('de_mirage', 'TopofMid'), 'Medio');
  assert.equal(label('de_mirage', ' TopofMid '), 'Medio');
  assert.equal(label('de_mirage', 'BombsiteA'), 'Bombsite A');
  assert.equal(label('de_nuke', 'TopofMid'), 'Top of Mid');
  assert.equal(label('de_mirage', 'el puente'), 'el puente');
});

test('a failed read keeps CS2 names and the next use reads again', async () => {
  let fail = true;
  const { loader, calls } = counting(async () => {
    if (fail) throw new Error('offline');
    return [{ token: 'BombsiteB', name: 'Bé' }];
  });
  const store = createCalloutNameStore(loader);
  await store.load('de_nuke');
  assert.equal(store.label()('de_nuke', 'BombsiteB'), 'Bombsite B');
  fail = false;
  await store.load('de_nuke');
  assert.equal(store.label()('de_nuke', 'BombsiteB'), 'Bé');
  assert.deepEqual(calls, ['de_nuke', 'de_nuke']);
});

test('a fresh read of the map wins over a slower one still on its way', async () => {
  let answer: (callouts: { token: string; name?: string }[]) => void = () => undefined;
  const store = createCalloutNameStore(() => new Promise((resolve) => (answer = resolve)));
  const slow = store.load('de_inferno');
  store.replace('de_inferno', [{ token: 'Banana', name: 'Plátano' }]);
  answer([{ token: 'Banana' }]);
  await slow;
  assert.equal(store.label()('de_inferno', 'Banana'), 'Plátano');
});

test('a rename tells every view and a reset forgets every map', async () => {
  const store = createCalloutNameStore(async () => [{ token: 'Apartments', name: 'Apps' }]);
  let notified = 0;
  const stop = store.subscribe(() => {
    notified += 1;
  });
  store.rename({ map: 'de_mirage', token: 'Apartments', name: 'Pisos' });
  assert.equal(notified, 0);
  await store.load('de_mirage');
  const before = store.label();
  store.rename({ map: 'de_mirage', token: 'Apartments', name: 'Pisos' });
  assert.notEqual(store.label(), before);
  assert.equal(store.label()('de_mirage', 'Apartments'), 'Pisos');
  store.rename({ map: 'de_mirage', token: 'Apartments', name: null });
  assert.equal(store.label()('de_mirage', 'Apartments'), 'Apartments');
  store.reset();
  assert.equal(notified, 4);
  stop();
  await store.load('de_mirage');
  assert.equal(store.label()('de_mirage', 'Apartments'), 'Apps');
  assert.equal(notified, 4);
});
