import {
  POSITIONS_FORMAT,
  POSITIONS_FRAME_HEAD_SIZE,
  POSITIONS_HEADER_SIZE,
  POSITIONS_MAGIC,
  POSITIONS_MAX_SLOTS,
  POSITIONS_SAMPLE_SIZE,
  POSITIONS_VERSION,
  POSITIONS_VERSION_V1,
} from './api/tactical.ts';
import type { TacticalFrame, TacticalRoundOffset, TacticalSample } from './api/tactical.ts';
import { equipmentById } from './tactical-equipment.ts';
import type { TacticalEquipment, TacticalInventoryItem } from './tactical-equipment.ts';

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
 *   frame, repeated
 *     int32  tick
 *     uint16 present mask, bit N set when slot N has a sample
 *     per present slot in ascending slot order:
 *       int16  x, int16 y, int16 z   quantised as origin + value * quantum
 *       uint16 yaw                   degrees * 65536 / 360
 *       uint8  health
 *       uint8  flags
 *       version 2 only, the gear block:
 *         uint8 field bits, then only the named fields in bit order
 *         1  uint8  pitch as (degrees + 90) * 254 / 180
 *         2  uint8  active item id (tactical-equipment.ts)
 *         4  uint8  n, then n bytes: id | (count - 1) << 6
 *         8  uint8  armor, uint8 kit bits (1 helmet, 2 defuser)
 *        16  uint16 money
 *        32  uint8  flash blindness left in tenths of a second
 *       A field is written when it changed since the slot's previous sample in
 *       the round, so a decode must start at a round's first frame.
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
      return 'El formato de posiciones no es compatible con esta versión de ClipHub.';
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
  if (version !== POSITIONS_VERSION && version !== POSITIONS_VERSION_V1) {
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
  const withGear = data.getUint16(6, true) >= POSITIONS_VERSION;
  const gear = new Map<number, SlotGear>();
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
      if (pos + POSITIONS_SAMPLE_SIZE > buffer.byteLength) {
        throw new TacticalDecodeError('truncated_data', `decode frames: truncated samples at byte ${pos}`);
      }
      const sample: TacticalSample = {
        slot,
        x: dequantize(data.getInt16(pos, true), scale.origin[0], scale.quantum),
        y: dequantize(data.getInt16(pos + 2, true), scale.origin[1], scale.quantum),
        z: dequantize(data.getInt16(pos + 4, true), scale.origin[2], scale.quantum),
        yaw: decodeYaw(data.getUint16(pos + 6, true)),
        health: data.getUint8(pos + 8),
        flags: data.getUint8(pos + 9),
      };
      pos += POSITIONS_SAMPLE_SIZE;
      if (withGear) pos = readGear(data, pos, sample, gear);
      samples.push(sample);
    }
    frames.push({ tick, samples });
  }
  return frames;
}

/** A slot's gear as of its last sample; replaced, never mutated, so samples may share the inventory. */
type SlotGear = {
  /** The quantised pitch byte. */
  pitchStep: number;
  activeWeapon: TacticalEquipment | null;
  inventory: readonly TacticalInventoryItem[];
  armor: number;
  helmet: boolean;
  defuser: boolean;
  money: number;
  flashed: number;
};

const EMPTY_GEAR: SlotGear = {
  pitchStep: 127, activeWeapon: null, inventory: [], armor: 0, helmet: false, defuser: false, money: 0, flashed: 0,
};

const GEAR_PITCH = 1;
const GEAR_WEAPON = 2;
const GEAR_INVENTORY = 4;
const GEAR_ARMOR = 8;
const GEAR_MONEY = 16;
const GEAR_FLASH = 32;

/** Reads one gear block at `pos` into `sample` and returns the position after it. */
function readGear(data: DataView, start: number, sample: TacticalSample, state: Map<number, SlotGear>): number {
  let pos = start;
  const need = (bytes: number): void => {
    if (pos + bytes > data.byteLength) {
      throw new TacticalDecodeError('truncated_data', `decode frames: truncated gear at byte ${pos}`);
    }
  };
  need(1);
  const bits = data.getUint8(pos);
  pos += 1;
  const next: SlotGear = { ...(state.get(sample.slot) ?? EMPTY_GEAR) };
  if ((bits & GEAR_PITCH) !== 0) {
    need(1);
    next.pitchStep = data.getUint8(pos);
    pos += 1;
  }
  if ((bits & GEAR_WEAPON) !== 0) {
    need(1);
    next.activeWeapon = equipmentById(data.getUint8(pos));
    pos += 1;
  }
  if ((bits & GEAR_INVENTORY) !== 0) {
    need(1);
    const count = data.getUint8(pos);
    pos += 1;
    need(count);
    const inventory: TacticalInventoryItem[] = [];
    for (let i = 0; i < count; i += 1) {
      const entry = data.getUint8(pos + i);
      const item = equipmentById(entry & 63);
      if (item !== null) inventory.push({ item, count: (entry >> 6) + 1 });
    }
    next.inventory = inventory;
    pos += count;
  }
  if ((bits & GEAR_ARMOR) !== 0) {
    need(2);
    next.armor = data.getUint8(pos);
    const kit = data.getUint8(pos + 1);
    next.helmet = (kit & 1) !== 0;
    next.defuser = (kit & 2) !== 0;
    pos += 2;
  }
  if ((bits & GEAR_MONEY) !== 0) {
    need(2);
    next.money = data.getUint16(pos, true);
    pos += 2;
  }
  if ((bits & GEAR_FLASH) !== 0) {
    need(1);
    next.flashed = data.getUint8(pos) / 10;
    pos += 1;
  }
  state.set(sample.slot, next);
  sample.pitch = (next.pitchStep * 180) / 254 - 90;
  sample.activeWeapon = next.activeWeapon;
  sample.inventory = next.inventory;
  sample.armor = next.armor;
  sample.helmet = next.helmet;
  sample.defuser = next.defuser;
  sample.money = next.money;
  sample.flashed = next.flashed;
  return pos;
}

/** Turns a quantised int16 step count back into a world coordinate. */
function dequantize(steps: number, origin: number, quantum: number): number {
  return origin + steps * quantum;
}

/** Turns the packed uint16 heading back into degrees in `[0, 360)`. */
function decodeYaw(packed: number): number {
  return (packed * 360) / YAW_STEPS;
}
