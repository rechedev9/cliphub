import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { FullDemoBumperOptions, FullDemoOptions } from '../full-demo-plan.ts';
import {
  availableFullDemoBumpers, FULL_DEMO_BUMPER_MEMORY_KEY, recallFullDemoBumpers, rememberFullDemoBumpers, withRememberedBumpers,
} from './full-demo-bumper-memory.ts';

const intro = { id: 'dddddddd-dddd-4ddd-8ddd-dddddddddddd', sha256: 'd'.repeat(64) };
const sponsor = { id: 'cccccccc-cccc-4ccc-8ccc-cccccccccccc', sha256: 'c'.repeat(64) };
const outro = { id: 'eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee', sha256: 'e'.repeat(64) };
const OFF = { enabled: false, video: null };
const all: FullDemoBumperOptions = { intro: { enabled: true, video: intro }, outro: { enabled: true, video: outro }, sponsor: { enabled: true, video: sponsor } };

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() { return values.size; },
    clear: () => values.clear(),
    getItem: (key) => values.get(key) ?? null,
    key: (index) => [...values.keys()][index] ?? null,
    removeItem: (key) => { values.delete(key); },
    setItem: (key, value) => { values.set(key, value); },
  };
}

test('the last clip choice is remembered and an empty choice forgets it', () => {
  const storage = memoryStorage();
  rememberFullDemoBumpers(all, storage);
  assert.deepEqual(recallFullDemoBumpers(storage), all);

  // A slot switched on without a clip, or switched off, is not remembered.
  rememberFullDemoBumpers({ intro: { enabled: true, video: null }, outro: { enabled: false, video: outro }, sponsor: { enabled: true, video: sponsor } }, storage);
  assert.deepEqual(recallFullDemoBumpers(storage), { intro: OFF, outro: OFF, sponsor: { enabled: true, video: sponsor } });

  rememberFullDemoBumpers({ intro: OFF, outro: OFF }, storage);
  assert.equal(storage.getItem(FULL_DEMO_BUMPER_MEMORY_KEY), null);
  assert.equal(recallFullDemoBumpers(storage), undefined);

  storage.setItem(FULL_DEMO_BUMPER_MEMORY_KEY, '{"intro":{"enabled":true,"video":{"id":"x"}}}');
  assert.equal(recallFullDemoBumpers(storage), undefined, 'a malformed memory is ignored');
  storage.setItem(FULL_DEMO_BUMPER_MEMORY_KEY, 'not json');
  assert.equal(recallFullDemoBumpers(storage), undefined);
});

test('clips deleted from the library or replaced by other bytes are dropped', async () => {
  const fetcher = (async (url: string | URL | Request) => {
    const path = String(url);
    if (path.endsWith(intro.id)) return new Response(null, { status: 404 });
    if (path.endsWith(sponsor.id)) return Response.json({ id: sponsor.id, sha256: 'f'.repeat(64) });
    return Response.json({ id: outro.id, sha256: outro.sha256, file_name: 'outro.mp4' });
  }) as typeof fetch;
  assert.deepEqual(await availableFullDemoBumpers(all, undefined, fetcher), { intro: OFF, outro: { enabled: true, video: outro } });

  const unreachable = (async () => { throw new TypeError('offline'); }) as typeof fetch;
  assert.deepEqual(await availableFullDemoBumpers(all, undefined, unreachable), all, 'an unverifiable clip is left to the planner');
  assert.equal(await availableFullDemoBumpers(undefined, undefined, fetcher), undefined);
});

test('remembered clips only fill defaults that have none', () => {
  const defaults = { bumpers: undefined } as unknown as FullDemoOptions;
  assert.deepEqual(withRememberedBumpers(defaults, all).bumpers, all);
  assert.equal(withRememberedBumpers(defaults, undefined), defaults);
  const chosen = { bumpers: { intro: { enabled: true, video: outro }, outro: OFF } } as unknown as FullDemoOptions;
  assert.equal(withRememberedBumpers(chosen, all), chosen);
});
