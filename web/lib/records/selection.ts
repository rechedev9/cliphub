/** A counted noun with its gender, so the words around it agree: "partida" is feminine, "archivo" is not. */
export type CountNoun = { readonly one: string; readonly many: string; readonly feminine: boolean };

/** "1 partida", "3 archivos". */
export function countOf(count: number, noun: CountNoun): string {
  return count === 1 ? `1 ${noun.one}` : `${count} ${noun.many}`;
}

/** "1 partida seleccionada", "3 archivos seleccionados". */
export function selectedCount(count: number, noun: CountNoun): string {
  const ending = (noun.feminine ? 'a' : 'o') + (count === 1 ? '' : 's');
  return `${countOf(count, noun)} seleccionad${ending}`;
}
