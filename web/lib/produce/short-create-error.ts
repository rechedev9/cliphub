import { classifyFullDemoLoadFailure } from '../full-demo.ts';
import { PRODUCE_SHORT_CREATE_ERROR, PRODUCE_SHORT_CREATE_OFFLINE } from './copy.ts';

/**
 * `readJson` stamps every API failure with a numeric `status`; the domain errors
 * `createVideo` throws on purpose are plain Errors with Spanish text and neither field.
 */
function isTransportOrServerFailure(err: unknown): boolean {
  if (err instanceof TypeError) return true;
  if (typeof err !== 'object' || err === null) return true;
  if ('status' in err && typeof err.status === 'number') return true;
  return 'code' in err && typeof err.code === 'string';
}

export function shortCreateErrorMessage(err: unknown): string {
  if (!isTransportOrServerFailure(err)) {
    return err instanceof Error && err.message ? err.message : PRODUCE_SHORT_CREATE_ERROR;
  }
  return classifyFullDemoLoadFailure(err) === 'offline' ? PRODUCE_SHORT_CREATE_OFFLINE : PRODUCE_SHORT_CREATE_ERROR;
}
