import assert from 'node:assert/strict';
import test from 'node:test';
import { DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import {
  applyMusicChoice,
  gameVolumePercentToRequest,
  gameVolumeRequestToPercent,
  musicChoicesEqual,
  musicVolumePercentToRequest,
  musicVolumeRequestToPercent,
  titleWithMusicSuffix,
} from './reel-music.ts';

function intent(over: Partial<ReelIntent> = {}): ReelIntent {
  return {
    videoId: 'v',
    jobId: 'j',
    segmentIds: ['s'],
    mode: 'clean',
    variant: 'viral-60-clean',
    editConfig: DEFAULT_EDIT_CONFIG,
    title: '8 jugadas - Rondas 9, 24 - Killfeed',
    map: 'Mirage',
    score: '',
    createdAt: 0,
    ...over,
  };
}

test('music choices treat missing volume as full volume and ignore volume without a song', () => {
  assert.equal(musicChoicesEqual({}, {}), true);
  assert.equal(musicChoicesEqual({ songId: 'a' }, { songId: 'a', musicVolume: 1 }), true);
  assert.equal(musicChoicesEqual({ songId: 'a', musicVolume: 0.35 }, { songId: 'a', musicVolume: 0.35 }), true);
  assert.equal(musicChoicesEqual({ songId: 'a' }, { songId: 'b' }), false);
  assert.equal(musicChoicesEqual({ songId: 'a' }, { songId: 'a', musicVolume: 0.35 }), false);
  assert.equal(musicChoicesEqual({}, { songId: 'a' }), false);
  assert.equal(musicChoicesEqual({ songId: 'a' }, { songId: 'a', gameVolume: 0.2 }), true);
  assert.equal(musicChoicesEqual({ songId: 'a', gameVolume: 0.2 }, { songId: 'a', gameVolume: 0.5 }), false);
});

test('volume percent conversion keeps full volume unset', () => {
  assert.equal(musicVolumePercentToRequest(100), undefined);
  assert.equal(musicVolumePercentToRequest(35), 0.35);
  assert.equal(musicVolumeRequestToPercent(undefined), 100);
  assert.equal(musicVolumeRequestToPercent(0.35), 35);
  assert.equal(gameVolumePercentToRequest(20), 0.2);
  assert.equal(gameVolumePercentToRequest(0), 0);
  assert.equal(gameVolumeRequestToPercent(undefined), 20);
  assert.equal(gameVolumeRequestToPercent(0.5), 50);
});

test('applying music writes mode and clears the approved cover', () => {
  const reel = intent({ selectedCoverName: 'cover-1.jpg' });
  applyMusicChoice(reel, { songId: 'phonk-01', musicVolume: 0.35, gameVolume: 0.2 });
  assert.equal(reel.mode, 'music');
  assert.equal(reel.songId, 'phonk-01');
  assert.equal(reel.musicVolume, 0.35);
  assert.equal(reel.gameVolume, 0.2);
  assert.equal(reel.selectedCoverName, undefined);

  applyMusicChoice(reel, { songId: 'phonk-01' });
  assert.equal(reel.musicVolume, undefined);
  assert.equal(reel.gameVolume, undefined);

  applyMusicChoice(reel, {});
  assert.equal(reel.mode, 'clean');
  assert.equal(reel.songId, undefined);
  assert.equal(reel.musicVolume, undefined);
  assert.equal(reel.gameVolume, undefined);
});

test('title suffix flips between clean and music without rewriting the selection label', () => {
  const clean = '8 jugadas - Rondas 9, 24 - Killfeed';
  const withMusic = '8 jugadas - Rondas 9, 24 - Killfeed + Music';
  assert.equal(titleWithMusicSuffix(clean, 'Killfeed', true), withMusic);
  assert.equal(titleWithMusicSuffix(withMusic, 'Killfeed', false), clean);
  assert.equal(titleWithMusicSuffix(clean, 'Killfeed', false), clean);
  assert.equal(titleWithMusicSuffix(withMusic, 'Killfeed', true), withMusic);
  assert.equal(titleWithMusicSuffix('Ace en Mirage', 'Killfeed', true), 'Ace en Mirage');
});
