import assert from 'node:assert/strict';
import test from 'node:test';
import { diagnosticLines, diagnosticText } from './diagnostic-summary.ts';

test('lists the identifiers in a fixed order and skips unknown ones', () => {
  const lines = diagnosticLines({
    route: '/clips/3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e',
    jobId: '3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e',
    supportCode: 'CH-AAAA-BBBB-CCCC-DDDD-EEEE',
    sessionId: '8d0f7a52-6f55-4e0c-9f0e-7b1e2f3a4b5c',
    digest: ' ',
  });
  assert.deepEqual(lines.map((line) => line.label), ['Código de soporte', 'Sesión', 'Trabajo', 'Ruta']);
  assert.equal(diagnosticText(lines), [
    'Código de soporte: CH-AAAA-BBBB-CCCC-DDDD-EEEE',
    'Sesión: 8d0f7a52-6f55-4e0c-9f0e-7b1e2f3a4b5c',
    'Trabajo: 3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e',
    'Ruta: /clips/3f2b8c1e-4d5a-4b6c-8d7e-9f0a1b2c3d4e',
  ].join('\n'));
});

test('an error boundary adds the digest and message', () => {
  const text = diagnosticText(diagnosticLines({ digest: '2345678901', route: '/clips', message: 'boom' }));
  assert.equal(text, 'Digest: 2345678901\nRuta: /clips\nMensaje: boom');
  assert.deepEqual(diagnosticLines({}), []);
});
