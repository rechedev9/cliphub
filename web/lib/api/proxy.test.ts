import assert from 'node:assert/strict';
import test from 'node:test';
import { NextRequest } from 'next/server.js';
import { config, proxy } from '../../proxy.ts';

function apiRequest(headers: Record<string, string>): NextRequest {
  return new NextRequest('http://127.0.0.1:3000/api/jobs', { headers });
}

test('proxy permits same-origin reads through the real Next boundary', async () => {
  const response = await proxy(apiRequest({
    host: '127.0.0.1:3000',
    origin: 'http://127.0.0.1:3000',
    'sec-fetch-site': 'same-origin',
  }));

  assert.equal(response.status, 200);
  assert.equal(response.headers.get('x-middleware-next'), '1');
});

test('proxy rejects cross-origin and DNS-rebound reads', async () => {
  const cases = [
    {
      headers: {
        host: '127.0.0.1:3000',
        origin: 'https://attacker.example',
        'sec-fetch-site': 'cross-site',
      },
      error: 'cross-site request blocked',
    },
    {
      headers: {
        host: 'attacker.example:3000',
        origin: 'http://attacker.example:3000',
        'sec-fetch-site': 'same-origin',
      },
      error: 'local API host rejected',
    },
  ];

  for (const { headers, error } of cases) {
    const response = await proxy(apiRequest(headers));
    assert.equal(response.status, 403);
    assert.deepEqual(await response.json(), { error });
  }
});

test('proxy preserves the API matcher and streaming-route exclusions', () => {
  assert.deepEqual(config, {
    matcher: '/api/((?!demos/scan/?$|streams/?$|editor/assets/?$|session/bootstrap/?$).*)',
  });
});
