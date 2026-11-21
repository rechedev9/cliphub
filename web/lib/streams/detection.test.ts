import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamVoiceScan } from '../api/streams.ts';
import { detectionNote, detectionSettingsChanged, scanVoiceSettings, shoutOptions } from './detection.ts';

function scanned(over: Partial<StreamVoiceScan> = {}): StreamVoiceScan {
  return { enabled: true, sensitivity: 3, status: 'done', peaks: 2, ...over };
}

test('shouting less finds more voice peaks, so Poco is the highest level', () => {
  assert.deepEqual(shoutOptions(3), [
    { value: '5', label: 'Poco' },
    { value: '3', label: 'Normal' },
    { value: '1', label: 'Mucho' },
  ]);
});

test('a level the assistant set between presets is offered in its place', () => {
  assert.deepEqual(shoutOptions(4), [
    { value: '5', label: 'Poco' },
    { value: '4', label: 'Nivel 4' },
    { value: '3', label: 'Normal' },
    { value: '1', label: 'Mucho' },
  ]);
});

test('the settings of the finished search are not a change', () => {
  assert.equal(detectionSettingsChanged(scanned(), { enabled: true, sensitivity: 3 }), false);
});

test('another level or turning voice peaks off is a change', () => {
  assert.equal(detectionSettingsChanged(scanned(), { enabled: true, sensitivity: 5 }), true);
  assert.equal(detectionSettingsChanged(scanned(), { enabled: false, sensitivity: 3 }), true);
});

test('the level does not matter while voice peaks stay off', () => {
  assert.equal(detectionSettingsChanged(scanned({ enabled: false }), { enabled: false, sensitivity: 1 }), false);
});

test('a search saved before voice peaks can always be redone', () => {
  assert.equal(detectionSettingsChanged(undefined, { enabled: true, sensitivity: 3 }), true);
});

test('a new search reuses the settings the last search ran with', () => {
  assert.deepEqual(scanVoiceSettings(scanned({ enabled: false, sensitivity: 5, status: 'off' })), { enabled: false, sensitivity: 5 });
  assert.deepEqual(scanVoiceSettings(scanned({ sensitivity: 1, mix: true })), { enabled: true, sensitivity: 1 });
});

test('a video never searched starts with voice peaks on at Normal', () => {
  assert.deepEqual(scanVoiceSettings(undefined), { enabled: true, sensitivity: 3 });
});

test('a search without a mic of its own says so without editor words', () => {
  const note = detectionNote(scanned({ mix: true })) ?? '';
  assert.match(note, /micro/);
  assert.doesNotMatch(note, /pista|mezcla|sensibilidad|rango/i);
});

test('a search that heard the mic has no note', () => {
  assert.equal(detectionNote(scanned()), null);
  assert.equal(detectionNote(undefined), null);
});

test('a video with no audio explains why there are no voice peaks', () => {
  assert.match(detectionNote(scanned({ status: 'no_audio' })) ?? '', /no tiene audio/);
});
