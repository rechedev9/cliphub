import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryMatchTeam } from './api/library.ts';
import type { StorageFile } from './api/storage.ts';
import { bulkDeleteWarning, groupByFolder, groupByKind, libraryCopyLabel, libraryVideoFor, matchLabel, splitPath } from './storage-files.ts';

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

test('libraryVideoFor finds the Mis vídeos card of a render MP4', () => {
  const reel = { ...file('D:/data/jobs/j1/renders/v/revisions/r/videos/reel.mp4', 1, 'short'), jobId: 'j1' };
  const videos = [
    { id: 'other', jobId: 'j2', downloadUrl: '/api/demos/j2/renders/v/videos/reel.mp4' },
    { id: 'mine', jobId: 'j1', downloadUrl: '/api/demos/j1/renders/v/videos/reel.mp4?revision=r' },
  ];
  assert.equal(libraryVideoFor(videos, reel), 'mine');
  assert.equal(libraryVideoFor(videos, { ...reel, kind: 'stream' }), null);
  assert.equal(libraryVideoFor([], reel), null);
});

test('bulkDeleteWarning only mentions what the selection holds', () => {
  assert.equal(bulkDeleteWarning([file('a.dem', 1)]), 'Los .dem elegidos se borran de tu PC.');
  assert.equal(
    bulkDeleteWarning([file('c.dem', 1, 'studio_demo'), file('v.mp4', 1, 'short')]),
    'Cada copia de Studio se borra con su partida, sus vídeos y sus capturas. Los vídeos elegidos se borran.',
  );
});

test('libraryCopyLabel names a library copy by its match, and gives up on unknown copies', () => {
  const team = (clan: string, score: number, teamName?: string): LibraryMatchTeam => ({
    key: 'ct-start',
    start_side: 'CT',
    clan_name: clan,
    team_id: null,
    team_name: teamName,
    assignment: 'auto',
    score,
  });
  const matches = [{ demo_id: 'd1', map: 'de_mirage', teams: [team('clanA', 13, 'Vitality'), team('clanB', 9)] }];
  assert.equal(libraryCopyLabel('C:\\data\\library\\d1\\demo.dem', matches), 'Mirage · Vitality 13-9 clanB');
  assert.equal(libraryCopyLabel('/data/library/d2/demo.dem', matches), null);
  assert.equal(libraryCopyLabel('C:\\data\\jobs\\d1\\demo.dem', matches), null);
});
