import assert from 'node:assert/strict';
import test from 'node:test';
import type { FaceitFollowedPlayer } from './api/faceit.ts';
import { defaultPlayerTab, followedPlayersReducer, headerFollowFailure, profileFollowFailure, type FollowedPlayersState, type FollowFailure } from './followed-players.ts';

const player = (id: string): FaceitFollowedPlayer => ({ id, nickname: id, profile_url: `https://www.faceit.com/en/players/${id}` });

for (const order of ['follow-first', 'unfollow-first'] as const) {
  test(`concurrent successful follow and unfollow retain both server results: ${order}`, () => {
    const initial: FollowedPlayersState = { players: [player('old'), player('other')], selectedID: 'old' };
    const before = structuredClone(initial);
    const follow = { type: 'followed', player: player('new') } as const;
    const unfollow = { type: 'unfollowed', id: 'old' } as const;
    // React queues both completions before committing a render. Each action
    // must consume the preceding queued state, never the last rendered roster.
    const actions = order === 'follow-first' ? [follow, unfollow] : [unfollow, follow];
    const result = actions.reduce(followedPlayersReducer, initial);
    assert.deepEqual(result.players.map((entry) => entry.id), ['new', 'other']);
    assert.equal(result.selectedID, 'new');
    assert.deepEqual(initial, before);
  });
}

test('unfollowing the selection chooses a remaining player or clears the last selection', () => {
  const initial = { players: [player('old'), player('other')], selectedID: 'old' };
  const remaining = followedPlayersReducer(initial, { type: 'unfollowed', id: 'old' });
  assert.equal(remaining.selectedID, 'other');
  assert.deepEqual(followedPlayersReducer(remaining, { type: 'unfollowed', id: 'other' }), { players: [], selectedID: null });
});

test('a delayed profile response cannot resurrect an unfollowed player or change the selection', () => {
  const initial = { players: [player('old'), player('other')], selectedID: 'old' };
  const removed = followedPlayersReducer(initial, { type: 'unfollowed', id: 'old' });
  assert.deepEqual(followedPlayersReducer(removed, { type: 'profile', player: player('old') }), removed);
});

test('the rail opens on Seguidos when the user follows anyone, otherwise on the ranking', () => {
  assert.equal(defaultPlayerTab([player('own')]), 'seguidos');
  assert.equal(defaultPlayerTab([]), 'ranking');
});

test('a live profile updates the followed row and keeps when it was followed', () => {
  const initial: FollowedPlayersState = { players: [{ ...player('pro'), followed_at: '2026-08-17T12:00:00Z' }], selectedID: 'pro' };
  const refreshed = followedPlayersReducer(initial, { type: 'profile', player: { ...player('pro'), elo: 4800 } });
  assert.deepEqual(refreshed.players, [{ ...player('pro'), elo: 4800, followed_at: '2026-08-17T12:00:00Z' }]);
});

test('a polled list always takes the followed player\'s stored ELO', () => {
  const own = { ...player('mine'), elo: 2100 };
  const live = followedPlayersReducer({ players: [own], selectedID: 'mine' }, { type: 'profile', player: { ...player('mine'), elo: 2000 } });
  const listed = followedPlayersReducer(live, { type: 'listed', players: [{ ...own, elo: 2150 }] });
  assert.deepEqual(listed, { players: [{ ...own, elo: 2150 }], selectedID: 'mine' });
});

test('a follow refused from a profile shows beside that profile, not in the header out of view', () => {
  const limit = 'Ya sigues a 20 jugadores, el máximo. Deja de seguir a alguno para añadir a otro.';
  const fromProfile: FollowFailure = { message: limit, profileID: 'pro' };
  assert.equal(profileFollowFailure(fromProfile, 'pro'), limit);
  assert.equal(profileFollowFailure(fromProfile, 'other'), undefined, 'another profile does not inherit it');
  assert.equal(headerFollowFailure(fromProfile), undefined);
  const fromForm: FollowFailure = { message: limit };
  assert.equal(headerFollowFailure(fromForm), limit, 'the search form in the header keeps its own message');
  assert.equal(profileFollowFailure(fromForm, 'pro'), undefined);
});
