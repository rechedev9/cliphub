import assert from 'node:assert/strict';
import test from 'node:test';
import type { FaceitFollowedPlayer } from './api/faceit.ts';
import { defaultPlayerTab, followedPlayersReducer, inPlayerTab, type FollowedPlayersState } from './followed-players.ts';

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
  assert.deepEqual(followedPlayersReducer(removed, { type: 'profile', player: player('old'), at: 1 }), removed);
});

test('tabs split own follows from the zone rosters, and a followed zone player shows in both', () => {
  const own = player('own');
  const seededCIS = { ...player('cis'), seeded: true, zone: 'cis' } as const;
  const followedLATAM = { ...player('latam'), zone: 'latam' } as const;
  const players = [own, seededCIS, followedLATAM];
  assert.deepEqual(players.filter((entry) => inPlayerTab(entry, 'custom')).map((entry) => entry.id), ['own', 'latam']);
  assert.deepEqual(players.filter((entry) => inPlayerTab(entry, 'cis')).map((entry) => entry.id), ['cis']);
  assert.deepEqual(players.filter((entry) => inPlayerTab(entry, 'latam')).map((entry) => entry.id), ['latam']);
  assert.equal(defaultPlayerTab(players), 'custom');
  assert.equal(defaultPlayerTab([seededCIS]), 'cis');
  assert.equal(defaultPlayerTab([]), 'custom');
});

test('following a zone player keeps the zone, and unfollowing returns them to it', () => {
  const initial: FollowedPlayersState = { players: [{ ...player('pro'), seeded: true, zone: 'cis' }], selectedID: 'pro' };
  const followed = followedPlayersReducer(initial, { type: 'followed', player: player('pro') });
  assert.deepEqual(followed.players, [{ ...player('pro'), zone: 'cis' }]);
  const unfollowed = followedPlayersReducer(followed, { type: 'unfollowed', id: 'pro' });
  assert.deepEqual(unfollowed, { players: [{ ...player('pro'), zone: 'cis', seeded: true }], selectedID: 'pro' });
  // Removing the seeded row itself is a dismissal.
  assert.deepEqual(followedPlayersReducer(unfollowed, { type: 'unfollowed', id: 'pro' }), { players: [], selectedID: null });
});

test('a live profile refresh keeps the row in its zone list', () => {
  const initial: FollowedPlayersState = { players: [{ ...player('pro'), seeded: true, zone: 'cis' }], selectedID: 'pro' };
  const refreshed = followedPlayersReducer(initial, { type: 'profile', player: { ...player('pro'), elo: 4800 }, at: 1000 });
  assert.deepEqual(refreshed.players, [{ ...player('pro'), elo: 4800, seeded: true, zone: 'cis', liveAt: 1000 }]);
});

test('a polled list keeps a newer live ELO and takes a newer roster ELO', () => {
  const seeded = { ...player('pro'), seeded: true, zone: 'cis', elo: 4700 } as const;
  const live = followedPlayersReducer({ players: [seeded], selectedID: 'pro' },
    { type: 'profile', player: { ...player('pro'), elo: 4800, skill_level: 10 }, at: 2000 });
  const staleList = followedPlayersReducer(live, { type: 'listed', players: [seeded], updatedAt: 1000 });
  assert.equal(staleList.players[0]?.elo, 4800);
  assert.equal(staleList.players[0]?.seeded, true);
  const freshList = followedPlayersReducer(staleList, { type: 'listed', players: [{ ...seeded, elo: 4825 }], updatedAt: 3000 });
  assert.deepEqual(freshList.players, [{ ...seeded, elo: 4825 }]);
});
