import assert from 'node:assert/strict';
import { test } from 'node:test';
import { DEFAULT_MAP_SETTINGS, parseMapSettings } from './map-settings.ts';

test('stored map settings keep what is valid and default the rest', () => {
  assert.deepEqual(parseMapSettings(null), DEFAULT_MAP_SETTINGS);
  assert.deepEqual(parseMapSettings('not json'), DEFAULT_MAP_SETTINGS);
  assert.deepEqual(parseMapSettings('{"art":"real","dots":"xl","names":"s"}'), { art: 'real', dots: 'm', names: 's', floors: 'both' });
  assert.equal(parseMapSettings('{"floors":"one"}').floors, 'one');
  assert.equal(parseMapSettings('{"floors":"three"}').floors, 'both');
});
