import assert from 'node:assert/strict';
import { test } from 'node:test';
import { registerWebTools, type WebTool } from './webmcp.ts';

const tool: WebTool = { name: 'read', description: 'Read', inputSchema: { type: 'object' }, execute: () => 'ok' };

test('an unavailable native API leaves the page operational', async (t) => {
  const warn = t.mock.method(console, 'warn', () => {});
  registerWebTools(undefined, [tool])();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(warn.mock.callCount(), 0);
});

test('unmount cancels registrations and prevents late registrations', async () => {
  const registered: AbortSignal[] = [];
  let finish: (() => void) | undefined;
  const stop = registerWebTools({ registerTool: (_tool, { signal }) => {
    registered.push(signal);
    return new Promise<void>(resolve => { finish = resolve; });
  } }, [tool, { ...tool, name: 'later' }]);
  assert.equal(registered.length, 1);
  stop();
  finish?.();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(registered.length, 1);
  assert.equal(registered[0].aborted, true);
});

test('registration preserves the application callback and schema', async () => {
  const registrations: WebTool[] = [];
  const stop = registerWebTools({ registerTool: value => { registrations.push(value); } }, [tool]);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(registrations[0], tool);
  stop();
});
