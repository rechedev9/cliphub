import assert from 'node:assert/strict';
import test from 'node:test';
import { ControlPlaneClient, type AgentRegistration } from './control-plane-client.ts';

const registration: AgentRegistration = {
  identity: {
    deviceId: '11111111-1111-4111-8111-111111111111',
    secret: 'a'.repeat(64),
  },
  name: 'Gaming PC',
  platform: 'windows-amd64',
  version: '2.5.0',
};

test('registers a pending device and validates the response', async () => {
  const requests: Array<{ url: string; init: RequestInit }> = [];
  const client = new ControlPlaneClient('https://cliphub.example', async (input, init) => {
    requests.push({ url: String(input), init: init ?? {} });
    return Response.json({
      pairing: {
        deviceId: registration.identity.deviceId,
        code: 'ABCDE23456',
        expiresAt: '2026-08-27T12:10:00Z',
      },
    }, { status: 201 });
  });

  const state = await client.register(registration);
  assert.deepEqual(state, { status: 'pending', code: 'ABCDE23456', expiresAt: '2026-08-27T12:10:00Z' });
  assert.equal(requests[0]?.url, 'https://cliphub.example/api/agent/pairings');
  assert.equal(requests[0]?.init.method, 'POST');
  assert.equal(String(requests[0]?.init.body).includes(registration.identity.secret), true);
});

test('recognizes an already claimed device without rotating its identity', async () => {
  let calls = 0;
  const client = new ControlPlaneClient('http://127.0.0.1:8090', async () => {
    calls += 1;
    if (calls === 1) return Response.json({ code: 'invalid_pairing' }, { status: 400 });
    return Response.json({ claimed: true });
  });
  assert.deepEqual(await client.register(registration), { status: 'claimed' });
  assert.equal(calls, 2);
});

test('heartbeat authenticates with the device secret and excludes it from the body', async () => {
  let request: RequestInit | undefined;
  const client = new ControlPlaneClient('https://cliphub.example', async (_input, init) => {
    request = init;
    return new Response(null, { status: 204 });
  });
  await client.heartbeat(registration);
  assert.equal((request?.headers as Record<string, string>).Authorization, `Bearer ${registration.identity.secret}`);
  assert.equal(String(request?.body).includes(registration.identity.secret), false);
});

test('rejects insecure remote control plane URLs and malformed responses', async () => {
  assert.throws(() => new ControlPlaneClient('http://cliphub.example'), /must use HTTPS/);
  const client = new ControlPlaneClient('https://cliphub.example', async () => Response.json({ pairing: { code: 'bad' } }));
  await assert.rejects(client.register(registration), /invalid pairing/);
});
