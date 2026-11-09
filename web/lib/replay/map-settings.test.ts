import assert from 'node:assert/strict';
import { test } from 'node:test';
import { DEFAULT_MAP_SETTINGS, namesShown, parseMapSettings, toggleNames } from './map-settings.ts';

test('stored map settings keep what is valid and default the rest', () => {
  assert.deepEqual(parseMapSettings(null), DEFAULT_MAP_SETTINGS);
  assert.deepEqual(parseMapSettings('not json'), DEFAULT_MAP_SETTINGS);
  assert.deepEqual(parseMapSettings('{"art":"real","dots":"xl","names":"s"}'), { ...DEFAULT_MAP_SETTINGS, art: 'real', names: 's' });
  assert.equal(parseMapSettings('{"floors":"one"}').floors, 'one');
  assert.equal(parseMapSettings('{"floors":"three"}').floors, 'both');
  assert.deepEqual(parseMapSettings('{"zoom":"l","fps":60}'), { ...DEFAULT_MAP_SETTINGS, zoom: 'l', fps: 60 });
  assert.equal(parseMapSettings('{"fps":45}').fps, 0);
});

test('each thing the map draws is kept on its own, and an unknown value stays on', () => {
  const settings = parseMapSettings('{"show":{"killfeed":false,"damage":"no","extra":false}}');
  assert.deepEqual(settings.show, { ...DEFAULT_MAP_SETTINGS.show, killfeed: false });
  assert.deepEqual(parseMapSettings('{"show":null}').show, DEFAULT_MAP_SETTINGS.show);
});

test('names are switched for the mode on screen only', () => {
  const multiOff = toggleNames(DEFAULT_MAP_SETTINGS, true);
  assert.equal(namesShown(multiOff, true), false);
  assert.equal(namesShown(multiOff, false), true);
  assert.deepEqual(parseMapSettings(JSON.stringify(multiOff)), multiOff);
});
