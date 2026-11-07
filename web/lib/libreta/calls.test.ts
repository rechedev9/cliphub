import assert from 'node:assert/strict';
import test from 'node:test';
import type { LibraryCall } from '../api/calls.ts';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '../records/table-view.ts';
import { CALL_FILTER_FIELDS, callSearchText, tagOptions } from './calls.ts';
import { visibleByStatus } from './status.ts';

function call(over: Partial<LibraryCall>): LibraryCall {
  return { id: over.name ?? 'x', name: 'x', side: 'both', meaning: '', status: 'active', tags: [], created_at: '', updated_at: '', ...over };
}

const calls = [
  call({ name: 'Rata', side: 'T', meaning: 'Uno se queda atrás\nhasta el final', tags: ['lurk'] }),
  call({ name: 'Stack B', side: 'CT', tags: ['anti-eco', 'retake'] }),
  call({ name: 'Eco', side: 'both', meaning: 'Nadie compra rifle', tags: ['economía'] }),
  call({ name: 'Viejo', side: 'T', status: 'archived' }),
];

function shown(filters: Record<string, string>): string[] {
  const filtered = filterRows(calls, filters, { fields: CALL_FILTER_FIELDS, searchText: callSearchText });
  return visibleByStatus(filtered, filters.status ?? FILTER_ANY, FILTER_ANY).map((entry) => entry.name);
}

test('a side keeps the calls of both sides, and Ambos only those', () => {
  assert.deepEqual(shown({ side: 'T' }), ['Rata', 'Eco']);
  assert.deepEqual(shown({ side: 'CT' }), ['Stack B', 'Eco']);
  assert.deepEqual(shown({ side: 'both' }), ['Eco']);
});

test('archived calls stay hidden until the status asks for them', () => {
  assert.deepEqual(shown({}), ['Rata', 'Stack B', 'Eco']);
  assert.deepEqual(shown({ status: 'archived' }), ['Viejo']);
});

test('tag and text filters read the tags and the meaning, without accents or case', () => {
  assert.deepEqual(shown({ tag: 'retake' }), ['Stack B']);
  assert.deepEqual(shown({ [SEARCH_FILTER]: 'economia' }), ['Eco']);
  assert.deepEqual(shown({ [SEARCH_FILTER]: 'hasta FINAL' }), ['Rata']);
  assert.deepEqual(
    tagOptions(calls).map((option) => option.value),
    ['anti-eco', 'economía', 'lurk', 'retake'],
  );
});
