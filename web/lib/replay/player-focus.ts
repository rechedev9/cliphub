/** The players the Replay puts in the foreground, by slot; empty means the whole team. */
export type PlayerFocus = readonly number[];

/** The URL key of the focus, the same one the heatmap and the overlay read. */
export const FOCUS_QUERY = 'subject';

/**
 * A click on a player: alone it leaves only that player in focus (or the whole
 * team again when they already were the only one); with Shift it adds or removes them.
 */
export function pickPlayer(focus: PlayerFocus, slot: number, additive: boolean): number[] {
  if (additive) {
    return focus.includes(slot) ? focus.filter((entry) => entry !== slot) : [...focus, slot].sort((a, b) => a - b);
  }
  return focus.length === 1 && focus[0] === slot ? [] : [slot];
}

/** Whether a player is in the foreground: everyone is while nobody is chosen. */
export function inFocus(focus: PlayerFocus, slot: number): boolean {
  return focus.length === 0 || focus.includes(slot);
}

export function focusFromQuery(params: URLSearchParams, slots: readonly number[]): number[] {
  const out: number[] = [];
  for (const raw of params.getAll(FOCUS_QUERY)) {
    const slot = Number(raw);
    if (Number.isInteger(slot) && slots.includes(slot) && !out.includes(slot)) out.push(slot);
  }
  return out.sort((a, b) => a - b);
}

/** The query with the focus written into it; the other keys stay. */
export function writeFocus(query: string, focus: PlayerFocus): string {
  const params = new URLSearchParams(query);
  params.delete(FOCUS_QUERY);
  for (const slot of focus) params.append(FOCUS_QUERY, String(slot));
  return params.toString();
}
