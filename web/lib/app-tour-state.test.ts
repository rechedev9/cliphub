import test from 'node:test';
import assert from 'node:assert/strict';
import {
  TOUR_SEEN_KEY,
  appTourSnapshot,
  closeAppTour,
  markTourSeen,
  openAppTour,
  reportTelemetryNotice,
  resetAppTour,
  shouldAutoOpenTour,
  subscribeToAppTour,
  tourWasSeen,
} from './app-tour-state.ts';

function memoryStorage(seed: Record<string, string> = {}): Pick<Storage, 'getItem' | 'setItem'> {
  const map = new Map(Object.entries(seed));
  return {
    getItem: (key) => map.get(key) ?? null,
    setItem: (key, value) => {
      map.set(key, value);
    },
  };
}

const throwingStorage: Pick<Storage, 'getItem' | 'setItem'> = {
  getItem: () => {
    throw new Error('SecurityError');
  },
  setItem: () => {
    throw new Error('QuotaExceededError');
  },
};

test('app tour: auto-opens only in Studio, once, after the telemetry notice is answered', () => {
  assert.equal(shouldAutoOpenTour({ desktop: true, seen: false, telemetryNotice: 'settled' }), true);
  assert.equal(shouldAutoOpenTour({ desktop: false, seen: false, telemetryNotice: 'settled' }), false);
  assert.equal(shouldAutoOpenTour({ desktop: true, seen: true, telemetryNotice: 'settled' }), false);
  assert.equal(shouldAutoOpenTour({ desktop: true, seen: false, telemetryNotice: 'open' }), false);
  assert.equal(shouldAutoOpenTour({ desktop: true, seen: false, telemetryNotice: 'pending' }), false);
});

test('app tour: seen flag round-trips through storage', () => {
  const storage = memoryStorage();
  assert.equal(tourWasSeen(storage), false);
  markTourSeen(storage, Date.UTC(2026, 8, 22));
  assert.equal(tourWasSeen(storage), true);
  assert.equal(storage.getItem(TOUR_SEEN_KEY), '2026-09-22T00:00:00.000Z');
});

test('app tour: unreadable storage counts as seen so the tour never loops', () => {
  assert.equal(tourWasSeen(null), true);
  assert.equal(tourWasSeen(throwingStorage), true);
  assert.doesNotThrow(() => markTourSeen(throwingStorage, 0));
});

test('app tour: store notifies only on real changes', () => {
  resetAppTour();
  let calls = 0;
  const unsubscribe = subscribeToAppTour(() => {
    calls += 1;
  });
  openAppTour(2);
  assert.deepEqual(appTourSnapshot(), { open: true, chapter: 2, telemetryNotice: 'pending' });
  openAppTour(2);
  assert.equal(calls, 1);
  reportTelemetryNotice('settled');
  closeAppTour();
  assert.deepEqual(appTourSnapshot(), { open: false, chapter: 2, telemetryNotice: 'settled' });
  assert.equal(calls, 3);
  unsubscribe();
  resetAppTour();
});
