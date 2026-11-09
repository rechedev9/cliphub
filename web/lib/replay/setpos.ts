/** One setpos line copied from the Replay: whose position, at what round clock, and the console line. */
export type SetposCopy = { id: number; name: string; at: string; command: string };

/** How many copied lines the player panel keeps. */
const SETPOS_KEPT = 6;

/** The list with a new copy on top; a line already there moves up instead of repeating. */
export function withSetpos(list: readonly SetposCopy[], entry: Omit<SetposCopy, 'id'>): SetposCopy[] {
  const id = list.reduce((top, item) => Math.max(top, item.id), 0) + 1;
  return [{ ...entry, id }, ...list.filter((item) => item.command !== entry.command)].slice(0, SETPOS_KEPT);
}
