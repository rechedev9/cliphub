import { MUTATION_CAPABILITY_ERROR } from '../api/errors.ts';
import { SERVICE_UNAVAILABLE_CODE } from '../api/types.ts';

const ORCHESTRATOR_DOWN = 'El orquestador no está en marcha.';
const MISSING_CREDENTIAL = 'Abre el Studio desde el launcher (falta la credencial local).';
const NO_ITEMS = 'Añade un clip al timeline antes de renderizar.';
const GENERIC = 'No se pudo completar la operación.';
const TIMELINE_NO_ITEMS = 'timeline has no items';

function readField(err: unknown, key: string): unknown {
  if (typeof err !== 'object' || err === null || !(key in err)) return undefined;
  return Reflect.get(err, key);
}

export function editorUserMessage(err: unknown): string {
  if (readField(err, 'code') === SERVICE_UNAVAILABLE_CODE) return ORCHESTRATOR_DOWN;
  const rawMessage = readField(err, 'message');
  const message = typeof rawMessage === 'string' ? rawMessage : '';
  const status = readField(err, 'status');
  if (message.includes(MUTATION_CAPABILITY_ERROR) || (status === 403 && message.toLowerCase().includes('capability'))) {
    return MISSING_CREDENTIAL;
  }
  if (message === TIMELINE_NO_ITEMS) return NO_ITEMS;
  if (message.trim() !== '') return message;
  return GENERIC;
}
