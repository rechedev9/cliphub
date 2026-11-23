import { countOf, type CountNoun } from '../records/selection.ts';

const NAMED = 3;

export function namesText(names: readonly string[]): string {
  const quoted = names.slice(0, NAMED).map((name) => `«${name}»`);
  const rest = names.length - quoted.length;
  if (rest > 0) return `${quoted.join(', ')} y ${rest} más`;
  if (quoted.length <= 1) return quoted.join('');
  return `${quoted.slice(0, -1).join(', ')} y ${quoted.at(-1) ?? ''}`;
}

function failureText({ names, noun, verb }: { names: readonly string[]; noun: CountNoun; verb: string }): string {
  const one = names.length === 1;
  const ending = (noun.feminine ? 'a' : 'o') + (one ? '' : 's');
  return `No se ${one ? 'pudo' : 'pudieron'} ${verb} ${namesText(names)}: ${one ? 'sigue' : 'siguen'} elegid${ending} para volver a intentarlo.`;
}

export function bulkDeleteMessage({ done, failed, noun }: { done: number; failed: readonly string[]; noun: CountNoun }): string {
  const ending = (noun.feminine ? 'a' : 'o') + (done === 1 ? '' : 's');
  const doneText = done > 0 ? `Borrad${ending} ${countOf(done, noun)}.` : '';
  if (failed.length === 0) return doneText;
  return [doneText, failureText({ names: failed, noun, verb: 'borrar' })].filter(Boolean).join(' ');
}

export function bulkStatusMessage({ done, failed, noun, status }: { done: number; failed: readonly string[]; noun: CountNoun; status: string }): string {
  const doneText = done > 0 ? `${countOf(done, noun)} ${done === 1 ? 'pasa' : 'pasan'} a ${status}.` : '';
  if (failed.length === 0) return doneText;
  return [doneText, failureText({ names: failed, noun, verb: 'cambiar' })].filter(Boolean).join(' ');
}
