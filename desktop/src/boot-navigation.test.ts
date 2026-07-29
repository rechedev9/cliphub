import assert from 'node:assert/strict';
import test from 'node:test';
import { isSupersededInternalNavigation } from './boot-navigation.ts';

test('accepts an aborted initial load replaced by a same-origin route', () => {
  const error = Object.assign(new Error("ERR_ABORTED (-3) loading 'http://127.0.0.1:3000/matches'"), {
    code: 'ERR_ABORTED',
    errno: -3,
  });
  assert.equal(
    isSupersededInternalNavigation(
      error,
      'http://127.0.0.1:3000/matches',
      {
        url: 'http://127.0.0.1:3000/settings',
        httpResponseCode: 200,
      },
      'http://127.0.0.1:3000',
    ),
    true,
  );
});

test('rejects non-abort failures and navigation outside the Studio origin', () => {
  assert.equal(
    isSupersededInternalNavigation(
      new Error('ERR_CONNECTION_REFUSED'),
      'http://127.0.0.1:3000/matches',
      {
        url: 'http://127.0.0.1:3000/settings',
        httpResponseCode: 200,
      },
      'http://127.0.0.1:3000',
    ),
    false,
  );
  assert.equal(
    isSupersededInternalNavigation(
      Object.assign(new Error('ERR_ABORTED (-3)'), { code: 'ERR_ABORTED' }),
      'http://127.0.0.1:3000/matches',
      {
        url: 'https://example.com/settings',
        httpResponseCode: 200,
      },
      'http://127.0.0.1:3000',
    ),
    false,
  );
});

test('rejects an abort without a distinct successful replacement navigation', () => {
  const error = Object.assign(new Error('ERR_ABORTED (-3)'), { code: 'ERR_ABORTED' });
  assert.equal(
    isSupersededInternalNavigation(
      error,
      'http://127.0.0.1:3000/matches',
      null,
      'http://127.0.0.1:3000',
    ),
    false,
  );
  assert.equal(
    isSupersededInternalNavigation(
      error,
      'http://127.0.0.1:3000/matches',
      {
        url: 'http://127.0.0.1:3000/matches',
        httpResponseCode: 200,
      },
      'http://127.0.0.1:3000',
    ),
    false,
  );
  assert.equal(
    isSupersededInternalNavigation(
      error,
      'http://127.0.0.1:3000/matches',
      {
        url: 'http://127.0.0.1:3000/settings',
        httpResponseCode: 500,
      },
      'http://127.0.0.1:3000',
    ),
    false,
  );
});
