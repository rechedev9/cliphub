import assert from 'node:assert/strict';
import test from 'node:test';
import { zipSync, strToU8 } from 'fflate';
import {
  archiveHasNoDemosError,
  archivePasswordError,
  isArchiveFileName,
  isDemoFileName,
  MAX_DEMO_FILES,
  notDemoOrArchiveError,
  tooManyDemosError,
} from './demo-names.ts';
import { expandDemoUploads } from './expand-archives.ts';

const DEMO_BYTES = strToU8('PBDEMS2');
const WITH_COMMENT_RAR = Buffer.from(
  'UmFyIRoHAM+QcwAADQAAAAAAAABE/XoAgCMAgAAAAHoAAAACz49u6RBWg0odMwMAAQAAAENNVAmRgUj+DP8lkhMHmASQ/weSuB6qBLpR5hAVgRbmhpQWpwFwlqcBRG9wBoQb3AUVFEaPLh/UcHHZN9gfx3H2G+QkNBsch2H4MKM+zftKitd/U8v3gxvoX2/UcRvxeGKIAjgjoh5Na88O461qTz+RPsmM0mwzF0ymRT9FY9y5doe1zHl0IJAuAAAAAAAAAAAAAgAAAAA1VYNKHTAJACAAAAAxRmlsZS50eHQAsCZjiozxdCCSNAAAAAAAAAAAAAIAAAAAOlWDSh0wDwAgAAAAMj8/LnR4dABOGzIth2UCALAgORXEPXsAQAcA',
  'base64',
);
const HEADER_ENC_RAR = Buffer.from(
  'UmFyIRoHAM6Zc4AADQAAAAAAAAAAAAAAAAAAAJPPq+tC7kb+20uYk0gq0MEW0RnMY87mJjVfk2pZ2l1hswZtLi3a3Tttx7+sAOo3yWhEDDlEV4/ZPa5HRtzk4ixxzAUX3NLdvGFtZzwNvrKER1U11CO58cEJYwMAb00mSQAAAAAAAAAAZe5YSqjdugsoKY2ljG5uBLezMVJhXr3/OEQAsi/rqqu2TzNN2+YlsYs7VgvvGm43ULoFeXNCHpwVtn+Ce+97gvXUeBs7k5xe4jIY084mD1tZyoG78cgcVKKhGnVpbN0DAAAAAAAAAACmAnZVHOHZezeKX5/8e6oS0CvlpebpmGzHcMoWvlyhcaDIZ38lgTkb5o2AbvKIpRYV1gIDXx+H2bbpACoTobXN7ge5vwHtUrbs64LyxHHoyjr/KtejMtZ69osAIX1DDpUAAAAAAAAAACQnI+yOgBdTJlP42jhLQRE=',
  'base64',
);

function namedFile(name: string, data: Uint8Array): File {
  const copy = new Uint8Array(data.byteLength);
  copy.set(data);
  return new File([copy], name, { type: 'application/octet-stream' });
}

const NAME_CASES: Array<{ name: string; demo: boolean; archive: boolean }> = [
  { name: 'match.dem', demo: true, archive: false },
  { name: 'match.DEM', demo: true, archive: false },
  { name: 'match.dem.zst', demo: true, archive: false },
  { name: 'match.dem.ZST', demo: true, archive: false },
  { name: 'series.rar', demo: false, archive: true },
  { name: 'series.RAR', demo: false, archive: true },
  { name: 'series.zip', demo: false, archive: true },
  { name: 'notes.txt', demo: false, archive: false },
  { name: 'match.dem.bak', demo: false, archive: false },
];

test('classifies demo and archive names', () => {
  for (const row of NAME_CASES) {
    assert.equal(isDemoFileName(row.name), row.demo, row.name);
    assert.equal(isArchiveFileName(row.name), row.archive, row.name);
  }
});

test('expands a zip of demos and ignores junk', async () => {
  const zip = zipSync({
    'series/m1-inferno.dem': DEMO_BYTES,
    'series/readme.txt': strToU8('nope'),
    '__MACOSX/._m1-inferno.dem': strToU8('apple'),
    'series/m2-mirage.dem.zst': DEMO_BYTES,
  });
  const result = await expandDemoUploads([namedFile('bo3.zip', zip)]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.deepEqual(
    result.files.map((file) => file.name).sort(),
    ['m1-inferno.dem', 'm2-mirage.dem.zst'],
  );
  assert.equal(await result.files[0]?.text(), 'PBDEMS2');
});

test('expands a rar of demos', async () => {
  const rar = buildStoreRar([
    { name: 'folder/m1-nuke.dem', data: DEMO_BYTES },
    { name: 'notes.txt', data: strToU8('nope') },
  ]);
  const result = await expandDemoUploads([namedFile('bo1.rar', rar)]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.deepEqual(
    result.files.map((file) => file.name),
    ['m1-nuke.dem'],
  );
});

test('sniffs a zip that was named .rar', async () => {
  const zip = zipSync({ 'solo.dem': DEMO_BYTES });
  const result = await expandDemoUploads([namedFile('misnamed.rar', zip)]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.deepEqual(
    result.files.map((file) => file.name),
    ['solo.dem'],
  );
});

test('keeps loose demos and merges extracted ones', async () => {
  const zip = zipSync({ 'm2-ancient.dem': DEMO_BYTES });
  const result = await expandDemoUploads([
    namedFile('m1-inferno.dem', DEMO_BYTES),
    namedFile('rest.zip', zip),
  ]);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.deepEqual(
    result.files.map((file) => file.name),
    ['m1-inferno.dem', 'm2-ancient.dem'],
  );
});

const REJECT_CASES: Array<{ name: string; files: File[]; error: string }> = [
  {
    name: 'unknown extension',
    files: [namedFile('clip.mp4', DEMO_BYTES)],
    error: notDemoOrArchiveError('clip.mp4'),
  },
  {
    name: 'zip without demos',
    files: [namedFile('empty.zip', zipSync({ 'readme.txt': strToU8('hi') }))],
    error: archiveHasNoDemosError('empty.zip'),
  },
  {
    name: 'rar without demos',
    files: [namedFile('comments.rar', new Uint8Array(WITH_COMMENT_RAR))],
    error: archiveHasNoDemosError('comments.rar'),
  },
  {
    name: 'password rar',
    files: [namedFile('secret.rar', new Uint8Array(HEADER_ENC_RAR))],
    error: archivePasswordError('secret.rar'),
  },
  {
    name: 'corrupt zip',
    files: [namedFile('bad.zip', strToU8('not-a-zip'))],
    error: 'No se pudo extraer "bad.zip".',
  },
  {
    name: 'corrupt rar',
    files: [namedFile('bad.rar', strToU8('not-a-rar'))],
    error: 'No se pudo extraer "bad.rar".',
  },
];

test('rejects unusable uploads', async () => {
  for (const row of REJECT_CASES) {
    const result = await expandDemoUploads(row.files);
    assert.equal(result.ok, false, row.name);
    if (result.ok) continue;
    assert.equal(result.error, row.error, row.name);
  }
});

test('caps the expanded demo count', async () => {
  const entries: Record<string, Uint8Array> = {};
  for (let i = 0; i < MAX_DEMO_FILES + 1; i += 1) {
    entries[`m${i}.dem`] = DEMO_BYTES;
  }
  const result = await expandDemoUploads([namedFile('too-many.zip', zipSync(entries))]);
  assert.deepEqual(result, { ok: false, error: tooManyDemosError(MAX_DEMO_FILES + 1) });
});

function crc32(data: Uint8Array): number {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc ^= byte;
    for (let i = 0; i < 8; i += 1) {
      crc = (crc >>> 1) ^ (crc & 1 ? 0xedb88320 : 0);
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function u16(n: number): Uint8Array {
  return Uint8Array.of(n & 0xff, (n >>> 8) & 0xff);
}

function u32(n: number): Uint8Array {
  return Uint8Array.of(n & 0xff, (n >>> 8) & 0xff, (n >>> 16) & 0xff, (n >>> 24) & 0xff);
}

function concatBytes(parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((sum, part) => sum + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

function rarHeader(type: number, flags: number, rest: Uint8Array): Uint8Array {
  const withoutCrc = concatBytes([Uint8Array.of(type), u16(flags), u16(7 + rest.length), rest]);
  return concatBytes([u16(crc32(withoutCrc) & 0xffff), withoutCrc]);
}

function buildStoreRar(entries: Array<{ name: string; data: Uint8Array }>): Uint8Array {
  const parts: Uint8Array[] = [
    Uint8Array.of(0x52, 0x61, 0x72, 0x21, 0x1a, 0x07, 0x00),
    rarHeader(0x73, 0, new Uint8Array(6)),
  ];
  for (const entry of entries) {
    const name = new TextEncoder().encode(entry.name);
    const rest = concatBytes([
      u32(entry.data.length),
      u32(entry.data.length),
      Uint8Array.of(2),
      u32(crc32(entry.data)),
      u32(0),
      Uint8Array.of(0x1d, 0x30),
      u16(name.length),
      u32(0x20),
      name,
    ]);
    parts.push(rarHeader(0x74, 0x8000, rest), entry.data);
  }
  parts.push(rarHeader(0x7b, 0x4000, new Uint8Array(0)));
  return concatBytes(parts);
}
