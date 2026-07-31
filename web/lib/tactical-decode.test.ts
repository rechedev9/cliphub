// Unit tests for the zvpos1 position-blob decoder.
// Run: pnpm --dir web run test:unit
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  decodeFrames,
  decodePositions,
  decodePositionsHeader,
  decodeRoundFrames,
  decodeRoundFramesFromSlice,
  TacticalDecodeError,
  tacticalDecodeErrorMessage,
} from './tactical-decode.ts';
import {
  POSITIONS_FRAME_HEAD_SIZE,
  POSITIONS_HEADER_SIZE,
  POSITIONS_SAMPLE_SIZE,
  TACTICAL_SAMPLE_FLAGS,
  hasSampleFlags,
} from './api/tactical.ts';
import type { TacticalRoundOffset } from './api/tactical.ts';

/*
 * The fixtures are written by hand against internal/tacticalplan/positions.go's
 * encoder, byte for byte: 32-byte header ("ZVPOS1", uint16 version, uint16 slot
 * count, uint16 sample ticks, float32 quantum, 3x float32 origin, uint32 frame
 * count), then per frame an int32 tick, a uint16 present mask, and one 10-byte
 * record per present slot in ascending slot order.
 */

type SampleSpec = {
  slot: number;
  x: number;
  y: number;
  z: number;
  yaw: number;
  health: number;
  flags: number;
};
type FrameSpec = { tick: number; samples: SampleSpec[] };
type RoundSpec = { round: number; frames: FrameSpec[] };
type BlobSpec = {
  quantum: number;
  origin: [number, number, number];
  slotCount: number;
  sampleTicks: number;
  rounds: RoundSpec[];
  magic?: string;
  version?: number;
  frameCount?: number;
};

const QUANTUM = 0.25;
// Every origin component is exactly representable as a float32, so a sample at
// origin + k*quantum decodes back to the very same number.
const ORIGIN: [number, number, number] = [-64.5, 128.25, -8];

function quantizeAxis(value: number, origin: number, quantum: number): number {
  const steps = Math.round((value - origin) / quantum);
  if (steps > 32767) return 32767;
  if (steps < -32768) return -32768;
  return steps;
}

function encodeYaw(degrees: number): number {
  let normalized = degrees % 360;
  if (normalized < 0) normalized += 360;
  return Math.round((normalized * 65536) / 360) & 0xffff;
}

function frameBytes(frame: FrameSpec): number {
  return POSITIONS_FRAME_HEAD_SIZE + frame.samples.length * POSITIONS_SAMPLE_SIZE;
}

function encodeBlob(spec: BlobSpec): { buffer: ArrayBuffer; offsets: TacticalRoundOffset[] } {
  const frames = spec.rounds.flatMap((round) => round.frames);
  const total = POSITIONS_HEADER_SIZE + frames.reduce((sum, frame) => sum + frameBytes(frame), 0);
  const buffer = new ArrayBuffer(total);
  const data = new DataView(buffer);

  const magic = spec.magic ?? 'ZVPOS1';
  for (let i = 0; i < 6; i += 1) data.setUint8(i, magic.charCodeAt(i));
  data.setUint16(6, spec.version ?? 1, true);
  data.setUint16(8, spec.slotCount, true);
  data.setUint16(10, spec.sampleTicks, true);
  data.setFloat32(12, spec.quantum, true);
  data.setFloat32(16, spec.origin[0], true);
  data.setFloat32(20, spec.origin[1], true);
  data.setFloat32(24, spec.origin[2], true);
  data.setUint32(28, spec.frameCount ?? frames.length, true);

  const offsets: TacticalRoundOffset[] = [];
  let pos = POSITIONS_HEADER_SIZE;
  for (const round of spec.rounds) {
    const start = pos;
    for (const frame of round.frames) {
      data.setInt32(pos, frame.tick, true);
      let mask = 0;
      for (const sample of frame.samples) mask |= 1 << sample.slot;
      data.setUint16(pos + 4, mask, true);
      pos += POSITIONS_FRAME_HEAD_SIZE;
      const ascending = [...frame.samples].sort((left, right) => left.slot - right.slot);
      for (const sample of ascending) {
        data.setInt16(pos, quantizeAxis(sample.x, spec.origin[0], spec.quantum), true);
        data.setInt16(pos + 2, quantizeAxis(sample.y, spec.origin[1], spec.quantum), true);
        data.setInt16(pos + 4, quantizeAxis(sample.z, spec.origin[2], spec.quantum), true);
        data.setUint16(pos + 6, encodeYaw(sample.yaw), true);
        data.setUint8(pos + 8, sample.health);
        data.setUint8(pos + 9, sample.flags);
        pos += POSITIONS_SAMPLE_SIZE;
      }
    }
    offsets.push({
      round: round.round,
      byte_offset: start,
      byte_length: pos - start,
      frame_count: round.frames.length,
      first_tick: round.frames[0]?.tick ?? 0,
      last_tick: round.frames[round.frames.length - 1]?.tick ?? 0,
    });
  }
  return { buffer, offsets };
}

function sample(slot: number, steps: [number, number, number], rest: Partial<SampleSpec> = {}): SampleSpec {
  return {
    slot,
    x: ORIGIN[0] + steps[0] * QUANTUM,
    y: ORIGIN[1] + steps[1] * QUANTUM,
    z: ORIGIN[2] + steps[2] * QUANTUM,
    yaw: rest.yaw ?? 0,
    health: rest.health ?? 100,
    flags: rest.flags ?? TACTICAL_SAMPLE_FLAGS.alive,
  };
}

function twoRoundBlob(): { buffer: ArrayBuffer; offsets: TacticalRoundOffset[] } {
  return encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 10,
    sampleTicks: 8,
    rounds: [
      {
        round: 1,
        frames: [
          {
            tick: 100,
            samples: [
              sample(0, [-6000, 4000, 12], { yaw: 90, health: 100, flags: TACTICAL_SAMPLE_FLAGS.alive }),
              sample(3, [1200, -900, -40], {
                yaw: 270,
                health: 56,
                flags: TACTICAL_SAMPLE_FLAGS.alive | TACTICAL_SAMPLE_FLAGS.sideT,
              }),
            ],
          },
          { tick: 108, samples: [sample(0, [-5990, 4010, 12], { yaw: 45, health: 100 })] },
        ],
      },
      {
        round: 2,
        frames: [
          {
            tick: 500,
            samples: [
              sample(9, [32000, -32000, 0], {
                yaw: 180,
                health: 0,
                flags: TACTICAL_SAMPLE_FLAGS.defusing | TACTICAL_SAMPLE_FLAGS.ducking,
              }),
            ],
          },
        ],
      },
    ],
  });
}

test('decodes the fixed header exactly as the Go encoder wrote it', () => {
  const { buffer } = twoRoundBlob();
  const header = decodePositionsHeader(buffer);
  assert.equal(header.format, 'zvpos1');
  assert.equal(header.version, 1);
  assert.equal(header.slotCount, 10);
  assert.equal(header.sampleTicks, 8);
  assert.equal(header.quantum, QUANTUM);
  assert.deepEqual(header.origin, ORIGIN);
  assert.equal(header.frameCount, 3);
  assert.equal(header.byteLength, buffer.byteLength);
});

test('decodes every frame, dequantising against the header origin and quantum', () => {
  const { buffer } = twoRoundBlob();
  const { header, frames } = decodePositions(buffer);
  assert.equal(frames.length, 3);
  assert.deepEqual(
    frames.map((frame) => frame.tick),
    [100, 108, 500],
  );

  const first = frames[0];
  assert.equal(first.samples.length, 2);
  // Ascending slot order comes from the present mask, not from the wire order.
  assert.deepEqual(
    first.samples.map((s) => s.slot),
    [0, 3],
  );
  assert.deepEqual(first.samples[0], {
    slot: 0,
    x: ORIGIN[0] + -6000 * QUANTUM,
    y: ORIGIN[1] + 4000 * QUANTUM,
    z: ORIGIN[2] + 12 * QUANTUM,
    yaw: 90,
    health: 100,
    flags: TACTICAL_SAMPLE_FLAGS.alive,
  });
  assert.equal(header.frameCount, frames.length);
});

test('decodes negative int16 step counts as world coordinates below the origin', () => {
  const { buffer } = twoRoundBlob();
  const { frames } = decodePositions(buffer);
  const slot9 = frames[2].samples[0];
  assert.equal(slot9.slot, 9);
  assert.equal(slot9.x, ORIGIN[0] + 32000 * QUANTUM);
  assert.equal(slot9.y, ORIGIN[1] - 32000 * QUANTUM);
  assert.ok(slot9.y < ORIGIN[1]);
});

test('decodes the flag byte into the documented bits', () => {
  const { buffer } = twoRoundBlob();
  const { frames } = decodePositions(buffer);
  const t = frames[0].samples[1];
  assert.equal(t.health, 56);
  assert.ok(hasSampleFlags(t.flags, TACTICAL_SAMPLE_FLAGS.alive));
  assert.ok(hasSampleFlags(t.flags, TACTICAL_SAMPLE_FLAGS.sideT));
  assert.ok(hasSampleFlags(t.flags, TACTICAL_SAMPLE_FLAGS.alive | TACTICAL_SAMPLE_FLAGS.sideT));
  assert.equal(hasSampleFlags(t.flags, TACTICAL_SAMPLE_FLAGS.blinded), false);

  const dead = frames[2].samples[0];
  assert.equal(hasSampleFlags(dead.flags, TACTICAL_SAMPLE_FLAGS.alive), false);
  assert.ok(hasSampleFlags(dead.flags, TACTICAL_SAMPLE_FLAGS.defusing));
  assert.ok(hasSampleFlags(dead.flags, TACTICAL_SAMPLE_FLAGS.ducking));
});

test('decodes yaw over the full circle, wrapping instead of overflowing', () => {
  const yaws = [0, 45, 90, 180, 270, 359.9999, -90];
  const { buffer } = encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 1,
    sampleTicks: 8,
    rounds: [
      {
        round: 1,
        frames: yaws.map((yaw, index) => ({
          tick: index,
          samples: [sample(0, [0, 0, 0], { yaw })],
        })),
      },
    ],
  });
  const { frames } = decodePositions(buffer);
  const decoded = frames.map((frame) => frame.samples[0].yaw);
  for (const yaw of decoded) {
    assert.ok(yaw >= 0 && yaw < 360, `yaw ${yaw} outside [0, 360)`);
  }
  assert.equal(decoded[0], 0);
  assert.equal(decoded[2], 90);
  assert.equal(decoded[3], 180);
  assert.equal(decoded[4], 270);
  // Just under a full turn rounds onto 65536, which wraps to 0.
  assert.equal(decoded[5], 0);
  // A negative yaw normalises before encoding, so -90 comes back as 270.
  assert.equal(decoded[6], 270);
});

test('decodes one round from its byte offset without reading the rest of the blob', () => {
  const { buffer, offsets } = twoRoundBlob();
  const header = decodePositionsHeader(buffer);
  const second = offsets[1];

  const frames = decodeRoundFrames(buffer, second, header);
  assert.equal(frames.length, 1);
  assert.equal(frames[0].tick, 500);
  assert.equal(frames[0].samples[0].slot, 9);

  // The same round, fetched as a Range slice that starts at its byte offset.
  const slice = buffer.slice(second.byte_offset, second.byte_offset + second.byte_length);
  assert.equal(slice.byteLength, second.byte_length);
  assert.deepEqual(decodeRoundFramesFromSlice(slice, second, header), frames);
});

test('decodes with the JSON descriptor scale as well as with the header', () => {
  const { buffer, offsets } = twoRoundBlob();
  const descriptor = { quantum: QUANTUM, origin: ORIGIN };
  assert.deepEqual(
    decodeRoundFrames(buffer, offsets[0], descriptor),
    decodeRoundFrames(buffer, offsets[0], decodePositionsHeader(buffer)),
  );
});

test('an empty stream still decodes', () => {
  const { buffer } = encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 0,
    sampleTicks: 8,
    rounds: [],
  });
  const { header, frames } = decodePositions(buffer);
  assert.equal(header.frameCount, 0);
  assert.deepEqual(frames, []);
});

test('rejects a buffer shorter than the header', () => {
  const { buffer } = twoRoundBlob();
  assert.throws(
    () => decodePositionsHeader(buffer.slice(0, 10)),
    (error: unknown) =>
      error instanceof TacticalDecodeError
      && error.code === 'short_header'
      && /shorter than the 32-byte header/.test(error.message),
  );
});

test('maps typed decoder failures to localized user-safe messages', () => {
  const technical = new TacticalDecodeError('unsupported_format', 'decode positions: bad magic "oops"');
  const message = tacticalDecodeErrorMessage(technical);
  assert.equal(message, 'El formato de posiciones no es compatible con esta versión de TickCut.');
  assert.equal(message.includes('bad magic'), false);
});

test('rejects a bad magic', () => {
  const { buffer } = encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 1,
    sampleTicks: 8,
    magic: 'XVPOS1',
    rounds: [],
  });
  assert.throws(() => decodePositionsHeader(buffer), /bad magic/);
});

test('rejects an unsupported blob version', () => {
  const { buffer } = encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 1,
    sampleTicks: 8,
    version: 99,
    rounds: [],
  });
  assert.throws(() => decodePositionsHeader(buffer), /unsupported blob version 99/);
});

test('rejects more slots than the present mask can address', () => {
  const { buffer } = encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 17,
    sampleTicks: 8,
    rounds: [],
  });
  assert.throws(() => decodePositionsHeader(buffer), /slot count 17 exceeds 16/);
});

test('rejects a non-positive quantum', () => {
  const { buffer } = encodeBlob({
    quantum: 0,
    origin: ORIGIN,
    slotCount: 1,
    sampleTicks: 8,
    rounds: [],
  });
  assert.throws(() => decodePositionsHeader(buffer), /quantum 0 must be positive/);
});

test('rejects truncated frames instead of decoding half of one', () => {
  const { buffer } = twoRoundBlob();
  assert.throws(
    () => decodePositions(buffer.slice(0, POSITIONS_HEADER_SIZE + 8)),
    /truncated samples/,
  );
  assert.throws(
    () => decodePositions(buffer.slice(0, POSITIONS_HEADER_SIZE + 3)),
    /truncated frame header/,
  );
});

test('rejects a frame count the blob cannot satisfy', () => {
  const { buffer } = encodeBlob({
    quantum: QUANTUM,
    origin: ORIGIN,
    slotCount: 1,
    sampleTicks: 8,
    frameCount: 5,
    rounds: [{ round: 1, frames: [{ tick: 1, samples: [sample(0, [0, 0, 0])] }] }],
  });
  assert.throws(() => decodePositions(buffer), /truncated frame header/);
});

test('rejects an offset outside the blob or inside the header', () => {
  const { buffer } = twoRoundBlob();
  const header = decodePositionsHeader(buffer);
  assert.throws(() => decodeFrames(buffer, buffer.byteLength + 1, 1, header), /outside the/);
  assert.throws(() => decodeFrames(buffer, 4, 1, header), /outside the/);
});

test('rejects a round slice shorter than the round it claims to carry', () => {
  const { buffer, offsets } = twoRoundBlob();
  const header = decodePositionsHeader(buffer);
  const round = offsets[0];
  const short = buffer.slice(round.byte_offset, round.byte_offset + round.byte_length - 1);
  assert.throws(() => decodeRoundFramesFromSlice(short, round, header), /slice is/);
});

test('rejects a descriptor with no quantum', () => {
  const { buffer, offsets } = twoRoundBlob();
  assert.throws(
    () => decodeRoundFrames(buffer, offsets[0], { quantum: 0, origin: ORIGIN }),
    /descriptor has no quantum/,
  );
});
