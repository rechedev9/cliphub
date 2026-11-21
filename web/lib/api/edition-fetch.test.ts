import assert from 'node:assert/strict';
import { afterEach, test } from 'node:test';
import { apiFetch, EditionApiError } from './edition-fetch.ts';

const realFetch = globalThis.fetch;
const realEdition = process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
const requested: string[] = [];

function useEdition(edition: 'web' | 'desktop'): void {
  process.env.NEXT_PUBLIC_CLIPHUB_EDITION = edition;
  globalThis.fetch = async (input) => {
    requested.push(String(input));
    return new Response('{}');
  };
}

afterEach(() => {
  globalThis.fetch = realFetch;
  process.env.NEXT_PUBLIC_CLIPHUB_EDITION = realEdition;
  requested.length = 0;
});

test('the web edition refuses Studio-only APIs before the network', async () => {
  useEdition('web');
  for (const path of ['/api/demos/jobs', '/api/imported-videos', '/api/streams', '/api/presets', '/api/demos/batch-status?ids=a']) {
    await assert.rejects(apiFetch(path), EditionApiError);
  }
  assert.deepEqual(requested, []);
});

test('the web edition reaches the shared library and its own ClipMind routes', async () => {
  useEdition('web');
  await apiFetch('/api/library/matches');
  await apiFetch('/api/cm/auth/me');
  assert.deepEqual(requested, ['/api/library/matches', '/api/cm/auth/me']);
});

test('Studio refuses the ClipMind-only routes and serves its own', async () => {
  useEdition('desktop');
  await assert.rejects(apiFetch('/api/cm/auth/me'), EditionApiError);
  await apiFetch('/api/streams');
  assert.deepEqual(requested, ['/api/streams']);
});

test('URLs outside the same-origin API pass through', async () => {
  useEdition('web');
  await apiFetch('/unrar.wasm');
  await apiFetch('https://cliphubstrats.example/api/demos/x');
  assert.deepEqual(requested, ['/unrar.wasm', 'https://cliphubstrats.example/api/demos/x']);
});
