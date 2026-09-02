import test from 'node:test';
import assert from 'node:assert/strict';
import { MUTATION_CAPABILITY_ERROR } from '../api/errors.ts';
import { SERVICE_UNAVAILABLE_CODE } from '../api/types.ts';
import { editorUserMessage } from './errors.ts';

test('editorUserMessage maps known failures', () => {
  const cases: { name: string; err: unknown; want: string }[] = [
    {
      name: 'orchestrator down',
      err: Object.assign(new Error('analysis service unavailable'), { code: SERVICE_UNAVAILABLE_CODE }),
      want: 'El orquestador no está en marcha.',
    },
    {
      name: 'mutation capability message',
      err: new Error(MUTATION_CAPABILITY_ERROR),
      want: 'Abre el Studio desde el launcher (falta la credencial local).',
    },
    {
      name: '403 capability',
      err: Object.assign(new Error('capability cookie missing'), { status: 403 }),
      want: 'Abre el Studio desde el launcher (falta la credencial local).',
    },
    {
      name: 'empty timeline',
      err: new Error('timeline has no items'),
      want: 'Añade un clip al timeline antes de renderizar.',
    },
    {
      name: 'passthrough',
      err: new Error('item clip-1 speed must be between 0.25 and 3'),
      want: 'item clip-1 speed must be between 0.25 and 3',
    },
    {
      name: 'generic fallback',
      err: null,
      want: 'No se pudo completar la operación.',
    },
    {
      name: 'blank message',
      err: new Error('   '),
      want: 'No se pudo completar la operación.',
    },
  ];
  for (const tc of cases) {
    assert.equal(editorUserMessage(tc.err), tc.want, tc.name);
  }
});
