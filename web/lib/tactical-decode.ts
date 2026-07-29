import {
  POSITIONS_FORMAT,
  POSITIONS_FRAME_HEAD_SIZE,
  POSITIONS_HEADER_SIZE,
  POSITIONS_MAGIC,
  POSITIONS_MAX_SLOTS,
  POSITIONS_SAMPLE_SIZE,
  POSITIONS_VERSION,
} from './api/tactical.ts';
import type { TacticalFrame, TacticalRoundOffset, TacticalSample } from './api/tactical.ts';

/**
 * Pure decoder for the zvpos1 position blob produced by
 * `internal/tacticalplan/positions.go`. The Go encoder is the spec; this module
 * is its mirror image and nothing else: no fetching, no state, no DOM.
 *
 * Layout, all little-endian:
 *
 *   header (32 bytes)
 *     0  6 bytes  magic "ZVPOS1"
 *     6  uint16   version
 *     8  uint16   slot count
 *    10  uint16   sample ticks
 *    12  float32  quantum (world units per step)
 *    16  float32  origin x
 *    20  float32  origin y
 *    24  float32  origin z
 *    28  uint32   frame count
 *
 *   frame (6 + 10 * present bytes), repeated
 *     int32  tick
 *     uint16 present mask, bit N set when slot N has a sample
 *     per present slot in ascending slot order:
 *       int16  x, int16 y, int16 z   quantised as origin + value * quantum
 *       uint16 yaw                   degrees * 65536 / 360
 *       uint8  health
 *       uint8  flags
 *
 * Every failure is an Error: a bad magic, an unsupported version, a truncated
 * frame or an offset outside the blob never yield a silent partial decode.
 */

/** The scale a quantised sample is decoded with; both the header and the JSON descriptor supply it. */
export type PositionsScale = {
  quantum: number;
  origin: readonly [number, number, number];
};

/** The fixed blob header, as read from the first 32 bytes. */
export type PositionsHeader = PositionsScale & {
  format: typeof POSITIONS_FORMAT;
  version: number;
  slotCount: number;
  sampleTicks: number;
  frameCount: number;
  /** Length of the buffer the header was read from, not of the whole blob when a Range was used. */
  byteLength: number;
};

/** A whole-blob decode: the header plus every frame it declares. */
export type DecodedPositions = {
  header: PositionsHeader;
  frames: TacticalFrame[];
};

export type TacticalDecodeErrorCode =
  | 'short_header'
  | 'unsupported_format'
  | 'invalid_descriptor'
  | 'invalid_offset'
  | 'truncated_data';

/** Stable decoder boundary: UI code maps codes, never raw parser details. */
export class TacticalDecodeError extends Error {
  readonly code: TacticalDecodeErrorCode;

  constructor(code: TacticalDecodeErrorCode, detail: string) {
    super(detail);
    this.name = 'TacticalDecodeError';
    this.code = code;
  }
}

export function tacticalDecodeErrorMessage(error: TacticalDecodeError): string {
  switch (error.code) {
    case 'short_header':
    case 'truncated_data':
      return 'El archivo de posiciones está incompleto. Repite el análisis táctico.';
    case 'unsupported_format':
      return 'El formato de posiciones no es compatible con esta versión de FragForge.';
    case 'invalid_descriptor':
    case 'invalid_offset':
      return 'Los datos de posiciones no son válidos. Repite el análisis táctico.';
  }
}

const YAW_STEPS = 65536;

function view(buffer: ArrayBuffer): DataView {
  return new DataView(buffer);
}

/**
 * Reads the fixed header. Rejects a buffer that is too short, carries the wrong
 * magic or version, declares more slots than the mask can address, or has a
 * non-positive quantum, because each of those would decode into plausible
 * nonsense.
 */
export function decodePositionsHeader(buffer: ArrayBuffer): PositionsHeader {
  if (buffer.byteLength < POSITIONS_HEADER_SIZE) {
    throw new TacticalDecodeError(
      'short_header',
      `decode positions: ${buffer.byteLength} bytes is shorter than the ${POSITIONS_HEADER_SIZE}-byte header`,
    );
  }
  const data = view(buffer);
  let magic = '';
  for (let i = 0; i < POSITIONS_MAGIC.length; i += 1) {
    magic += String.fromCharCode(data.getUint8(i));
  }
  if (magic !== POSITIONS_MAGIC) {
    throw new TacticalDecodeError('unsupported_format', `decode positions: bad magic "${magic}"`);
  }
  const version = data.getUint16(6, true);
  if (version !== POSITIONS_VERSION) {
    throw new TacticalDecodeError('unsupported_format', `decode positions: unsupported blob version ${version}`);
  }
  const slotCount = data.getUint16(8, true);
  if (slotCount > POSITIONS_MAX_SLOTS) {
    throw new TacticalDecodeError(
      'invalid_descriptor',
      `decode positions: slot count ${slotCount} exceeds ${POSITIONS_MAX_SLOTS}`,
    );
  }
  const quantum = data.getFloat32(12, true);
  if (!(quantum > 0)) {
    throw new TacticalDecodeError('invalid_descriptor', `decode positions: quantum ${quantum} must be positive`);
  }
  return {
    format: POSITIONS_FORMAT,
    version,
    slotCount,
    sampleTicks: data.getUint16(10, true),
    quantum,
    origin: [data.getFloat32(16, true), data.getFloat32(20, true), data.getFloat32(24, true)],
    frameCount: data.getUint32(28, true),
    byteLength: buffer.byteLength,
  };
}

/**
 * Decodes `frameCount` frames starting at `byteOffset` inside a full blob, which
 * is what a round's `byte_offset` gives a caller that wants one round and
 * nothing else. The offset must land after the header and inside the buffer.
 */
export function decodeFrames(
  buffer: ArrayBuffer,
  byteOffset: number,
  frameCount: number,
  scale: PositionsScale,
): TacticalFrame[] {
  if (byteOffset < POSITIONS_HEADER_SIZE || byteOffset > buffer.byteLength) {
    throw new TacticalDecodeError(
      'invalid_offset',
      `decode frames: offset ${byteOffset} is outside the ${buffer.byteLength}-byte blob`,
    );
  }
  return readFrames(buffer, byteOffset, frameCount, scale);
}

/**
 * Decodes one round from a full blob, using only the bytes its offset points at.
 */
export function decodeRoundFrames(
  buffer: ArrayBuffer,
  offset: TacticalRoundOffset,
  scale: PositionsScale,
): TacticalFrame[] {
  return decodeFrames(buffer, offset.byte_offset, offset.frame_count, scale);
}

/**
 * Decodes one round from a slice fetched with `Range: bytes=byte_offset-…`, so
 * a viewer can draw a round without ever holding the rest of the blob. The slice
 * must start exactly at the round's `byte_offset`.
 */
export function decodeRoundFramesFromSlice(
  slice: ArrayBuffer,
  offset: TacticalRoundOffset,
  scale: PositionsScale,
): TacticalFrame[] {
  if (slice.byteLength < offset.byte_length) {
    throw new TacticalDecodeError(
      'truncated_data',
      `decode frames: round ${offset.round} slice is ${slice.byteLength} bytes, expected ${offset.byte_length}`,
    );
  }
  return readFrames(slice, 0, offset.frame_count, scale);
}

/** Decodes a whole blob: the header, then every frame it declares. */
export function decodePositions(buffer: ArrayBuffer): DecodedPositions {
  const header = decodePositionsHeader(buffer);
  return { header, frames: readFrames(buffer, POSITIONS_HEADER_SIZE, header.frameCount, header) };
}

function readFrames(
  buffer: ArrayBuffer,
  start: number,
  frameCount: number,
  scale: PositionsScale,
): TacticalFrame[] {
  if (!(scale.quantum > 0)) {
    throw new TacticalDecodeError('invalid_descriptor', 'decode frames: descriptor has no quantum');
  }
  if (!Number.isInteger(frameCount) || frameCount < 0) {
    throw new TacticalDecodeError(
      'invalid_descriptor',
      `decode frames: frame count ${frameCount} must be a non-negative integer`,
    );
  }
  const data = view(buffer);
  const frames: TacticalFrame[] = [];
  let pos = start;
  for (let i = 0; i < frameCount; i += 1) {
    if (pos + POSITIONS_FRAME_HEAD_SIZE > buffer.byteLength) {
      throw new TacticalDecodeError('truncated_data', `decode frames: truncated frame header at byte ${pos}`);
    }
    const tick = data.getInt32(pos, true);
    const mask = data.getUint16(pos + 4, true);
    pos += POSITIONS_FRAME_HEAD_SIZE;

    let present = 0;
    for (let slot = 0; slot < POSITIONS_MAX_SLOTS; slot += 1) {
      if ((mask & (1 << slot)) !== 0) present += 1;
    }
    if (pos + present * POSITIONS_SAMPLE_SIZE > buffer.byteLength) {
      throw new TacticalDecodeError('truncated_data', `decode frames: truncated samples at byte ${pos}`);
    }

    const samples: TacticalSample[] = [];
    // Samples are written in ascending slot order, so the mask alone says which
    // slot each fixed-size record belongs to.
    for (let slot = 0; slot < POSITIONS_MAX_SLOTS; slot += 1) {
      if ((mask & (1 << slot)) === 0) continue;
      samples.push({
        slot,
        x: dequantize(data.getInt16(pos, true), scale.origin[0], scale.quantum),
        y: dequantize(data.getInt16(pos + 2, true), scale.origin[1], scale.quantum),
        z: dequantize(data.getInt16(pos + 4, true), scale.origin[2], scale.quantum),
        yaw: decodeYaw(data.getUint16(pos + 6, true)),
        health: data.getUint8(pos + 8),
        flags: data.getUint8(pos + 9),
      });
      pos += POSITIONS_SAMPLE_SIZE;
    }
    frames.push({ tick, samples });
  }
  return frames;
}

/** Turns a quantised int16 step count back into a world coordinate. */
export function dequantize(steps: number, origin: number, quantum: number): number {
  return origin + steps * quantum;
}

/** Turns the packed uint16 heading back into degrees in `[0, 360)`. */
export function decodeYaw(packed: number): number {
  return (packed * 360) / YAW_STEPS;
}
