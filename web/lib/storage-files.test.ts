import test from 'node:test';
import assert from 'node:assert/strict';
import type { StorageFile } from './api/storage.ts';
import { groupByFolder, groupByKind, matchLabel, splitPath } from './storage-files.ts';

const file = (path: string, sizeBytes: number, kind: StorageFile['kind'] = 'demo'): StorageFile => ({
  path,
  sizeBytes,
  modifiedAt: '',
  kind,
});

test('splitPath handles Windows and POSIX separators', () => {
  assert.deepEqual(splitPath('C:\\Users\\me\\a.dem'), { folder: 'C:\\Users\\me', name: 'a.dem' });
  assert.deepEqual(splitPath('/home/me/a.dem'), { folder: '/home/me', name: 'a.dem' });
  assert.deepEqual(splitPath('a.dem'), { folder: '', name: 'a.dem' });
});

test('groupByFolder puts the heaviest folder first and sums it', () => {
  const groups = groupByFolder([
    file('C:\\Downloads\\a.dem', 10),
    file('D:\\replays\\b.dem', 30),
    file('C:\\Downloads\\c.dem', 5),
  ]);
  assert.deepEqual(groups.map((group) => [group.label, group.totalBytes, group.files.length]), [
    ['D:\\replays', 30, 1],
    ['C:\\Downloads', 15, 2],
  ]);
});

test('groupByKind labels each group in Spanish', () => {
  const groups = groupByKind([file('x/videos/a.mp4', 1, 'short'), file('y/videos/b.mp4', 9, 'full_demo')]);
  assert.deepEqual(groups.map((group) => group.label), ['Demo completa', 'Short']);
});

test('matchLabel names a known match and returns null otherwise', () => {
  const matches = new Map([['j1', { map: 'de_mirage', player: 'ana' }]]);
  assert.equal(matchLabel('j1', matches), 'Mirage · ana');
  assert.equal(matchLabel('j2', matches), null);
  assert.equal(matchLabel(undefined, matches), null);
});
