import test from 'node:test';
import assert from 'node:assert/strict';
import { searchOptions } from './entity-search.ts';

const OPTIONS = [{ label: 'FUT Esports' }, { label: 'Natus Vincere' }, { label: 'Ñandú' }];

test('the search ignores case and accents and an empty query keeps everything', () => {
  assert.deepEqual(searchOptions(OPTIONS, 'fut'), [OPTIONS[0]]);
  assert.deepEqual(searchOptions(OPTIONS, ' VINCERE '), [OPTIONS[1]]);
  assert.deepEqual(searchOptions(OPTIONS, 'nandu'), [OPTIONS[2]]);
  assert.equal(searchOptions(OPTIONS, '').length, 3);
  assert.equal(searchOptions(OPTIONS, 'zzz').length, 0);
});
