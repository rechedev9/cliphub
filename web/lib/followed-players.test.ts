import assert from 'node:assert/strict';
import test from 'node:test';
import type { FaceitFollowedPlayer } from './api/faceit.ts';
import { followedPlayersReducer, type FollowedPlayersState } from './followed-players.ts';

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
