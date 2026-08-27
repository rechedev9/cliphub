import assert from 'node:assert/strict';
import test from 'node:test';
import { configuredControlPlaneURL, isHostedPublicPath, isHostedWebMode } from './hosted-mode.ts';

test('hosted mode is explicit and does not infer from ambient hostnames', () => {
  assert.equal(isHostedWebMode({ CLIPHUB_WEB_MODE: 'hosted' }), true);
  assert.equal(isHostedWebMode({ CLIPHUB_WEB_MODE: 'local' }), false);
  assert.equal(isHostedWebMode({}), false);
});

test('control plane URL accepts only absolute HTTP origins and strips paths', () => {
  assert.equal(configuredControlPlaneURL({ CLIPHUB_CONTROL_PLANE_URL: 'http://127.0.0.1:8090/internal' })?.toString(), 'http://127.0.0.1:8090/');
  assert.equal(configuredControlPlaneURL({ CLIPHUB_CONTROL_PLANE_URL: 'file:///tmp/control' }), null);
  assert.equal(configuredControlPlaneURL({}), null);
});

test('only authentication, connection, and control APIs are public', () => {
  const cases = [
    { path: '/login', want: true },
    { path: '/register', want: true },
    { path: '/connect', want: true },
    { path: '/api/account/session', want: true },
    { path: '/api/agent/heartbeat', want: true },
    { path: '/api/installer', want: true },
    { path: '/generated/agent-sw.js', want: true },
    { path: '/onboarding', want: false },
    { path: '/api/demos/jobs', want: false },
  ];
  for (const tc of cases) {
    assert.equal(isHostedPublicPath(tc.path), tc.want, tc.path);
  }
});
