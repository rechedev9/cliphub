import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamPlay } from '../api/streams.ts';
import { summaryBulkChoice, summaryChoiceText, summaryDefaultIds } from './summary.ts';

function kill(id: string, start: number, end: number): StreamPlay {
  return { id, label: id, start_seconds: start, end_seconds: end, kill_seconds: [start + 2], score: 50 };
}

function shout(id: string, start: number, end: number): StreamPlay {
  return { ...kill(id, start, end), kind: 'voice_peak', kill_seconds: [], voice_seconds: [start + 1] };
}

const plays = [kill('a', 10, 20), shout('v', 30, 36), kill('b', 50, 60)];

test('the resumen starts with every jugada with a kill and leaves picos de voz out', () => {
  assert.deepEqual(summaryDefaultIds(plays), ['a', 'b']);
});

test('the bulk button chooses the same jugadas as the default choice', () => {
  const choice = summaryBulkChoice(new Set(['a']), plays);
  assert.equal(choice.label, 'Elegir las 2 con kill');
  assert.deepEqual([...choice.next].sort(), summaryDefaultIds(plays));
});

test('the bulk button keeps a pico de voz the user chose', () => {
  assert.deepEqual([...summaryBulkChoice(new Set(['v']), plays).next].sort(), ['a', 'b', 'v']);
});

test('the bulk button clears the choice once every jugada with a kill is in', () => {
  const choice = summaryBulkChoice(new Set(['a', 'b']), plays);
  assert.equal(choice.label, 'Quitar todas');
  assert.deepEqual([...choice.next], []);
});

test('the resumen text counts jugadas with a kill and picos de voz apart', () => {
  assert.equal(summaryChoiceText([kill('a', 10, 20), kill('b', 50, 60)]), 'Un vídeo 16:9 con 2 jugadas con kill, una tras otra, unos 0:20.');
  assert.equal(
    summaryChoiceText([kill('a', 10, 20), shout('v', 30, 36)]),
    'Un vídeo 16:9 con 1 jugada con kill y 1 pico de voz, una tras otra, unos 0:16.',
  );
  assert.equal(summaryChoiceText([shout('v', 30, 36), shout('w', 40, 44)]), 'Un vídeo 16:9 con 2 picos de voz, una tras otra, unos 0:10.');
  assert.equal(summaryChoiceText([]), 'Elige al menos una jugada.');
});

test('the bulk button names a single jugada with a kill', () => {
  assert.equal(summaryBulkChoice(new Set(), [kill('a', 0, 5), shout('v', 9, 12)]).label, 'Elegir la jugada con kill');
});
