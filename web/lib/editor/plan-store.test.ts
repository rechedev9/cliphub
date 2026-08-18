import test from 'node:test';
import assert from 'node:assert/strict';
import { defaultEditorDocument, type EditorDocument } from './evaluate.ts';
import { addItem } from './document.ts';
import { createPlanStore } from './plan-store.ts';

const ASSET = { id: '11111111-1111-1111-1111-111111111111', probe: { duration_seconds: 2 } };

function docA(): EditorDocument {
  return addItem(defaultEditorDocument(), 'v1', ASSET, 0);
}

function docB(): EditorDocument {
  return addItem(docA(), 'v1', ASSET, 2);
}

function fakeTimers(): {
  schedule: (fn: () => void, ms: number) => number;
  cancel: (id: number) => void;
  fire(): void;
  pending(): number;
} {
  const timers: { id: number; fn: () => void }[] = [];
  let nextId = 1;
  return {
    schedule(fn) {
      const id = nextId;
      nextId += 1;
      timers.push({ id, fn });
      return id;
    },
    cancel(id) {
      const index = timers.findIndex((timer) => timer.id === id);
      if (index >= 0) timers.splice(index, 1);
    },
    fire() {
      const timer = timers.shift();
      if (timer !== undefined) timer.fn();
    },
    pending() {
      return timers.length;
    },
  };
}

test('update then flush saves immediately', async () => {
  const calls: EditorDocument[] = [];
  const store = createPlanStore({
    putPlan: async (doc) => {
      calls.push(doc);
      return doc;
    },
    schedule: () => 1,
    cancel: () => undefined,
  });
  const doc = docA();
  store.update(doc);
  assert.equal(store.getState().dirty, true);
  await store.flush();
  assert.deepEqual(calls, [doc]);
  assert.equal(store.getState().dirty, false);
  assert.equal(store.getState().lastError, null);
});

test('two updates collapse to one put', async () => {
  const cases: { name: string; via: 'flush' | 'timer' }[] = [
    { name: 'flush', via: 'flush' },
    { name: 'timer', via: 'timer' },
  ];
  for (const tc of cases) {
    const calls: EditorDocument[] = [];
    const timers = fakeTimers();
    const store = createPlanStore({
      putPlan: async (doc) => {
        calls.push(doc);
        return doc;
      },
      schedule: timers.schedule,
      cancel: timers.cancel,
    });
    const first = docA();
    const second = docB();
    store.update(first);
    store.update(second);
    assert.equal(timers.pending(), 1, tc.name);
    if (tc.via === 'flush') await store.flush();
    else {
      timers.fire();
      await Promise.resolve();
      await Promise.resolve();
    }
    assert.deepEqual(calls, [second], tc.name);
  }
});

test('flush with no changes does not call putPlan', async () => {
  let calls = 0;
  const store = createPlanStore({
    putPlan: async (doc) => {
      calls += 1;
      return doc;
    },
  });
  await store.flush();
  assert.equal(calls, 0);
  store.update(docA());
  await store.flush();
  assert.equal(calls, 1);
  await store.flush();
  assert.equal(calls, 1);
});

test('putPlan reject leaves dirty and sets lastError', async () => {
  let calls = 0;
  const store = createPlanStore({
    putPlan: async () => {
      calls += 1;
      throw new Error('disk full');
    },
  });
  store.update(docA());
  await store.flush();
  assert.equal(calls, 2);
  assert.equal(store.getState().dirty, true);
  assert.equal(store.getState().lastError, 'disk full');
});

test('flush retries once then succeeds', async () => {
  let calls = 0;
  const store = createPlanStore({
    putPlan: async (doc) => {
      calls += 1;
      if (calls === 1) throw new Error('temp');
      return doc;
    },
  });
  store.update(docA());
  await store.flush();
  assert.equal(calls, 2);
  assert.equal(store.getState().dirty, false);
  assert.equal(store.getState().lastError, null);
});

test('lock drops updates', async () => {
  const calls: EditorDocument[] = [];
  const store = createPlanStore({
    putPlan: async (doc) => {
      calls.push(doc);
      return doc;
    },
  });
  store.lock();
  store.update(docA());
  await store.flush();
  assert.deepEqual(calls, []);
  assert.equal(store.getState().dirty, false);
  assert.equal(store.getState().locked, true);
  store.unlock();
  store.update(docB());
  await store.flush();
  assert.equal(calls.length, 1);
  assert.equal(store.getState().locked, false);
});

test('flush waits for in-flight save then writes latest', async () => {
  const calls: EditorDocument[] = [];
  let release: ((doc: EditorDocument) => void) | undefined;
  const first = docA();
  const second = docB();
  const store = createPlanStore({
    putPlan: (doc) => {
      calls.push(doc);
      if (calls.length === 1) {
        return new Promise((resolve) => {
          release = resolve;
        });
      }
      return Promise.resolve(doc);
    },
  });
  store.update(first);
  const flushing = store.flush();
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(store.getState().saving, true);
  store.update(second);
  assert.ok(release);
  if (release === undefined) return;
  release(first);
  await flushing;
  assert.deepEqual(calls, [first, second]);
  assert.equal(store.getState().dirty, false);
});
