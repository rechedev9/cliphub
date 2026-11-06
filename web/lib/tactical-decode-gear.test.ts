// The web decoder against a version 2 blob the Go encoder wrote. Regenerate the
// fixture with: go test ./internal/tacticalplan -run TestWebGearFixture -update-gear-fixture
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { decodePositionsHeader, decodeRoundFrames } from './tactical-decode.ts';
import { equipmentById, isGrenade, TACTICAL_EQUIPMENT } from './tactical-equipment.ts';
import type { TacticalFrame, TacticalRoundOffset, TacticalSample } from './api/tactical.ts';

type GoInventoryItem = { item: string; count: number };
type GoGear = {
  active_weapon: string;
  inventory: GoInventoryItem[] | null;
  armor: number;
  helmet: boolean;
  defuser: boolean;
  money: number;
  flash_seconds: number;
};
type GoSample = Omit<TacticalSample, 'pitch' | 'activeWeapon' | 'inventory'> & { pitch: number; gear: GoGear };
type GoFixture = {
  blob_base64: string;
  round_offsets: TacticalRoundOffset[];
  frames: { tick: number; samples: GoSample[] }[];
};

const fixture: GoFixture = JSON.parse(
  readFileSync(new URL('./testdata/zvpos-gear-fixture.json', import.meta.url), 'utf8'),
);

function fixtureBuffer(): ArrayBuffer {
  const bytes = Buffer.from(fixture.blob_base64, 'base64');
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength);
}

function decodeAll(): TacticalFrame[] {
  const buffer = fixtureBuffer();
  const header = decodePositionsHeader(buffer);
  return fixture.round_offsets.flatMap((offset) => decodeRoundFrames(buffer, offset, header));
}

test('decodes the gear the Go encoder wrote, round by round', () => {
  const frames = decodeAll();
  assert.equal(frames.length, fixture.frames.length);
  frames.forEach((frame, i) => {
    const want = fixture.frames[i];
    assert.equal(frame.tick, want.tick);
    assert.equal(frame.samples.length, want.samples.length);
    frame.samples.forEach((got, j) => {
      const goSample = want.samples[j];
      assert.equal(got.slot, goSample.slot);
      assert.equal(got.health, goSample.health);
      assert.equal(got.flags, goSample.flags);
      assert.ok(Math.abs((got.pitch ?? Number.NaN) - goSample.pitch) < 1e-9, `pitch ${got.pitch} vs ${goSample.pitch}`);
      assert.equal(got.activeWeapon, goSample.gear.active_weapon === '' ? null : goSample.gear.active_weapon);
      assert.deepEqual(got.inventory, goSample.gear.inventory ?? []);
      assert.equal(got.armor, goSample.gear.armor);
      assert.equal(got.helmet, goSample.gear.helmet);
      assert.equal(got.defuser, goSample.gear.defuser);
      assert.equal(got.money, goSample.gear.money);
      assert.ok(Math.abs((got.flashed ?? Number.NaN) - goSample.gear.flash_seconds) < 1e-9);
    });
  });
});

test('the fixture covers a full loadout and a flashed player', () => {
  const [first, , third] = decodeAll();
  const t = first.samples[0];
  assert.equal(t.activeWeapon, 'ak47');
  assert.deepEqual(t.inventory?.find((entry) => entry.item === 'flashbang'), { item: 'flashbang', count: 2 });
  assert.equal(t.helmet, true);
  assert.equal(t.money, 3150);
  assert.equal(first.samples[1].defuser, true);
  assert.equal(third.samples[0].activeWeapon, 'flashbang');
  assert.equal(third.samples[0].flashed, 2.3);
});

test('a round seeked alone carries its full gear', () => {
  const buffer = fixtureBuffer();
  const header = decodePositionsHeader(buffer);
  const last = fixture.round_offsets[fixture.round_offsets.length - 1];
  const [frame] = decodeRoundFrames(buffer, last, header);
  assert.equal(frame.samples[0].activeWeapon, 'ak47');
  assert.equal(frame.samples[0].armor, 100);
});

test('a truncated gear block is a decode error, not a partial sample', () => {
  const buffer = fixtureBuffer();
  const header = decodePositionsHeader(buffer);
  // The last round holds one sample; cut inside its inventory list.
  const last = fixture.round_offsets[fixture.round_offsets.length - 1];
  const cut = buffer.slice(0, last.byte_offset + 6 + 10 + 4);
  assert.throws(() => decodeRoundFrames(cut, last, header), /truncated gear/);
});

test('equipment ids name CS2 items and unknown ids read as null', () => {
  assert.equal(equipmentById(0), null);
  assert.equal(equipmentById(TACTICAL_EQUIPMENT.length), null);
  assert.equal(equipmentById(TACTICAL_EQUIPMENT.indexOf('m4a1_silencer')), 'm4a1_silencer');
  assert.ok(TACTICAL_EQUIPMENT.length <= 64, 'inventory bytes address 64 ids');
  assert.equal(isGrenade('incgrenade'), true);
  assert.equal(isGrenade('c4'), false);
});
