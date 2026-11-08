import assert from 'node:assert/strict';
import test from 'node:test';
import type { TacticalDocument } from '../api/tactical.ts';
import { LoadedAnalysis, framesDecodeError } from './loaded-analysis.ts';

function header(): ArrayBuffer {
  const buffer = new ArrayBuffer(32);
  const data = new DataView(buffer);
  for (let i = 0; i < 6; i += 1) data.setUint8(i, 'ZVPOS1'.charCodeAt(i));
  data.setUint16(6, 2, true);
  data.setUint16(8, 10, true);
  data.setUint16(10, 8, true);
  data.setFloat32(12, 1, true);
  return buffer;
}

function analysis(): LoadedAnalysis {
  const offsets = [{ round: 3, byte_offset: 999, byte_length: 10, frame_count: 1, first_tick: 0, last_tick: 0 }];
  const doc = { rounds: [], positions: { round_offsets: offsets } } as unknown as TacticalDocument;
  return new LoadedAnalysis(doc, header());
}

test('loaded analysis: a round that does not decode says why, and is not decoded again', () => {
  const loaded = analysis();
  const first = loaded.framesFor(3);
  assert.equal(first.length, 0);
  assert.equal(framesDecodeError(first), 'Los datos de posiciones no son válidos. Repite el análisis táctico.');
  assert.equal(loaded.framesFor(3), first);
});

test('loaded analysis: a round with no stored positions is empty, not broken', () => {
  const frames = analysis().framesFor(7);
  assert.equal(frames.length, 0);
  assert.equal(framesDecodeError(frames), null);
});
