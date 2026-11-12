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

async function inWebEdition<T>(run: () => Promise<T>): Promise<T> {
  const previous = process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
  process.env.NEXT_PUBLIC_CLIPHUB_EDITION = 'web';
  try {
    return await run();
  } finally {
    if (previous === undefined) delete process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
    else process.env.NEXT_PUBLIC_CLIPHUB_EDITION = previous;
  }
}

/** An access token the proxy can read the expiry of; the server checks the signature. */
function accessToken(expiresInMs: number): string {
  const payload = Buffer.from(JSON.stringify({ uid: 'u', sid: 's', ws: 'w', exp: Date.now() + expiresInMs })).toString('base64url');
  return `${payload}.signature`;
}

function signedIn(path: string, init: { method?: string; headers?: Record<string, string> } = {}): NextRequest {
  return new NextRequest(`https://clipmind.test${path}`, {
    method: init.method,
    headers: { host: 'clipmind.test', cookie: `cm_access=${accessToken(10 * 60_000)}; cm_refresh=r`, ...init.headers },
  });
}

test('proxy: the web edition redirects desktop pages to the download page', async () => {
  await inWebEdition(async () => {
    for (const path of ['/shorts', '/shorts/abc', '/mis-videos', '/jugadores/faceit', '/resultados/old?x=1']) {
      const response = await proxy(signedIn(path));
      assert.equal(response.status, 307, path);
      const location = response.headers.get('location') ?? '';
      assert.equal(new URL(location).pathname, '/descargar', path);
      assert.equal(new URL(location).search, '', path);
    }
  });
});

test('proxy: the web edition answers 404 for desktop API prefixes', async () => {
  await inWebEdition(async () => {
    const response = await proxy(signedIn('/api/demos/scan', { method: 'POST' }));
    assert.equal(response.status, 404);
    assert.deepEqual(await response.json(), { error: 'not available in this edition' });
  });
});

test('proxy: the web edition sends a visitor without a sign-in to the login page', async () => {
  await inWebEdition(async () => {
    const page = await proxy(new NextRequest('https://clipmind.test/partidas?map=de_dust2', { headers: { host: 'clipmind.test' } }));
    assert.equal(page.status, 307);
    const location = new URL(page.headers.get('location') ?? '');
    assert.equal(location.pathname, '/iniciar-sesion');
    assert.equal(location.searchParams.get('next'), '/partidas?map=de_dust2');

    const api = await proxy(new NextRequest('https://clipmind.test/api/library/matches', { headers: { host: 'clipmind.test' } }));
    assert.equal(api.status, 401);
    assert.deepEqual(await api.json(), { error: 'sign in required', code: 'unauthorized' });
  });
});

test('proxy: the web edition lets the account forms through and clears a dead sign-in', async () => {
  await inWebEdition(async () => {
    for (const path of ['/iniciar-sesion', '/crear-cuenta', '/confirmar', '/recuperar', '/restablecer', '/descargar', '/api/cm/auth/login']) {
      const response = await proxy(new NextRequest(`https://clipmind.test${path}`, { headers: { host: 'clipmind.test' } }));
      assert.equal(response.headers.get('x-middleware-next'), '1', path);
    }
    // Expired access token and no refresh token: the cookie is dropped with the redirect.
    const expired = await proxy(new NextRequest('https://clipmind.test/partidas', {
      headers: { host: 'clipmind.test', cookie: `cm_access=${accessToken(-60_000)}` },
    }));
    assert.equal(expired.status, 307);
    assert.match(expired.headers.get('set-cookie') ?? '', /cm_access=;/);
  });
});

test('proxy: the web edition hands a live token to the handlers and discards one from the browser', async () => {
  await inWebEdition(async () => {
    const response = await proxy(signedIn('/api/library/matches', { headers: { 'x-cm-access': 'forged' } }));
    assert.equal(response.headers.get('x-middleware-next'), '1');
    const forwarded = response.headers.get('x-middleware-request-x-cm-access') ?? '';
    assert.notEqual(forwarded, 'forged');
    assert.match(forwarded, /^[A-Za-z0-9_-]+\.signature$/);

    const publicPage = await proxy(new NextRequest('https://clipmind.test/iniciar-sesion', { headers: { host: 'clipmind.test', 'x-cm-access': 'forged' } }));
    assert.equal(publicPage.headers.get('x-middleware-request-x-cm-access') ?? '', '');
  });
});

test('proxy: the web edition refuses a cross-site mutation', async () => {
  await inWebEdition(async () => {
    const response = await proxy(signedIn('/api/library/teams', {
      method: 'POST',
      headers: { origin: 'https://attacker.example', 'sec-fetch-site': 'cross-site' },
    }));
    assert.equal(response.status, 403);
    const sameOrigin = await proxy(signedIn('/api/library/teams', {
      method: 'POST',
      headers: { origin: 'https://clipmind.test', 'sec-fetch-site': 'same-origin' },
    }));
    assert.equal(sameOrigin.headers.get('x-middleware-next'), '1');
  });
});

test('proxy: the desktop edition has no account pages or account API', async () => {
  for (const path of ['/api/cm/auth/login', '/api/cm/workspaces']) {
    const response = await proxy(new NextRequest(`http://127.0.0.1:3000${path}`));
    assert.equal(response.status, 404, path);
  }
  const page = await proxy(new NextRequest('http://127.0.0.1:3000/iniciar-sesion'));
  assert.equal(page.status, 307);
  assert.equal(new URL(page.headers.get('location') ?? '').pathname, '/');
});
