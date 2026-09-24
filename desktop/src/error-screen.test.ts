import assert from 'node:assert/strict';
import test from 'node:test';
import { errorScreenHtml, RETRY_URL, SEND_DIAGNOSTIC_URL, type ErrorScreenInput } from './error-screen.ts';

const base: ErrorScreenInput = {
  error: new Error('orchestrator terminó inesperadamente (código 1)'),
  logFile: 'C:\\Users\\x\\AppData\\Roaming\\cliphub-studio\\studio.log',
  logTail: '[orchestrator] <panic> & more',
  send: 'idle',
  consented: false,
  supportCode: 'CH-AAAA-BBBB-CCCC-DDDD-EEEE',
};

test('offers retry and an explained send action before consent', () => {
  const html = errorScreenHtml(base);
  assert.ok(html.includes(`href="${RETRY_URL}"`));
  assert.ok(html.includes(`href="${SEND_DIAGNOSTIC_URL}">Enviar este diagnóstico</a>`));
  assert.match(html, /Al enviarlo se activan los diagnósticos/);
  assert.ok(html.includes('&lt;panic&gt; &amp; more'), 'log tail is escaped');
  assert.ok(html.includes("default-src 'none'"), 'no script can run on the error screen');
});

test('tells a consented user the button only sends what is pending', () => {
  const html = errorScreenHtml({ ...base, consented: true });
  assert.match(html, /ya están activados/);
  assert.doesNotMatch(html, /Al enviarlo se activan/);
});

test('hides the send action when the build has no collector', () => {
  const html = errorScreenHtml({ ...base, send: 'unavailable' });
  assert.equal(html.includes(SEND_DIAGNOSTIC_URL), false);
  assert.ok(html.includes(RETRY_URL));
});

test('shows progress and the support code after sending', () => {
  const sending = errorScreenHtml({ ...base, send: 'sending' });
  assert.equal(sending.includes(SEND_DIAGNOSTIC_URL), false);
  assert.match(sending, /Enviando…/);

  assert.match(errorScreenHtml({ ...base, send: 'sent' }), /Enviado · código CH-AAAA-BBBB-CCCC-DDDD-EEEE/);
  assert.match(errorScreenHtml({ ...base, send: 'queued' }), /en cuanto haya conexión · código CH-AAAA/);
  const failed = errorScreenHtml({ ...base, send: 'failed' });
  assert.match(failed, /no se ha enviado nada/);
  assert.ok(failed.includes(SEND_DIAGNOSTIC_URL), 'a failed consent save can be retried');
});

test('escapes titles, hints and the error', () => {
  const html = errorScreenHtml({ ...base, error: '<img src=x>', title: 'a<b', hint: 'c"d' });
  assert.ok(html.includes('&lt;img src=x&gt;'));
  assert.ok(html.includes('a&lt;b'));
  assert.ok(html.includes('c&quot;d'));
});
