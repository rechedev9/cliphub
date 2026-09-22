import assert from 'node:assert/strict';
import test from 'node:test';
import { faceitAvatarSrc, hasFaceitAvatar, playerInitial } from './faceit-avatar.ts';

test('playerInitial prefers the first letter, then the first digit, then "?"', () => {
  const cases: Array<{ nickname: string; want: string }> = [
    { nickname: 'donk666', want: 'D' },
    { nickname: '-SYPHO', want: 'S' },
    { nickname: '1769-', want: '1' },
    { nickname: '73ddd', want: 'D' },
    { nickname: '_ñu', want: 'Ñ' },
    { nickname: '---', want: '?' },
    { nickname: '', want: '?' },
  ];
  for (const { nickname, want } of cases) {
    assert.equal(playerInitial(nickname), want, nickname);
  }
});

test('hasFaceitAvatar only trusts a non-empty avatar URL', () => {
  assert.equal(hasFaceitAvatar({ avatar: 'https://cdn.faceit.com/a.jpg' }), true);
  assert.equal(hasFaceitAvatar({ avatar: '  ' }), false);
  assert.equal(hasFaceitAvatar({}), false);
});

test('faceitAvatarSrc encodes the player id', () => {
  assert.equal(faceitAvatarSrc('a b'), '/api/faceit/players/a%20b/avatar');
});
