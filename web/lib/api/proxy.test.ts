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
    matcher: [
      '/api/:path*',
      '/((?!api|_next/|favicon|icon\\.svg|.*\\..*).*)',
    ],
  });
});

test('proxy: excluded API routes keep their own in-handler guard', async () => {
  for (const path of ['/api/streams', '/api/editor/assets', '/api/library/demos', '/api/session/bootstrap']) {
    const response = await proxy(new NextRequest(`http://127.0.0.1:3000${path}`, {
      headers: { host: 'attacker.example:3000' },
    }));
    assert.equal(response.headers.get('x-middleware-next'), '1', path);
  }
});

test('proxy: the web edition redirects desktop pages to the download page', async () => {
  const previous = process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
  process.env.NEXT_PUBLIC_CLIPHUB_EDITION = 'web';
  try {
    for (const path of ['/shorts', '/shorts/abc', '/mis-videos', '/jugadores/faceit', '/resultados/old?x=1']) {
      const response = await proxy(new NextRequest(`http://127.0.0.1:3000${path}`));
      assert.equal(response.status, 307, path);
      const location = response.headers.get('location') ?? '';
      assert.equal(new URL(location).pathname, '/descargar', path);
      assert.equal(new URL(location).search, '', path);
    }
    const allowed = await proxy(new NextRequest('http://127.0.0.1:3000/partidas'));
    assert.equal(allowed.headers.get('x-middleware-next'), '1');
  } finally {
    if (previous === undefined) delete process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
    else process.env.NEXT_PUBLIC_CLIPHUB_EDITION = previous;
  }
});

test('proxy: the web edition answers 404 for desktop API prefixes', async () => {
  const previous = process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
  process.env.NEXT_PUBLIC_CLIPHUB_EDITION = 'web';
  try {
    const response = await proxy(new NextRequest('http://127.0.0.1:3000/api/demos/scan', { method: 'POST' }));
    assert.equal(response.status, 404);
    assert.deepEqual(await response.json(), { error: 'not available in this edition' });

    const allowed = await proxy(new NextRequest('http://127.0.0.1:3000/api/library/demos'));
    assert.equal(allowed.headers.get('x-middleware-next'), '1');
  } finally {
    if (previous === undefined) delete process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
    else process.env.NEXT_PUBLIC_CLIPHUB_EDITION = previous;
  }
});
