/**
 * Where the assistant lives, and who is allowed to know it.
 *
 * Two separate facts, deliberately kept apart:
 *
 * 1. WHETHER there is room for a permanent rail. This is pure CSS
 *    (`ASSISTANT_WIDE_QUERY`), never React state. The old code answered it with
 *    `useSyncExternalStore(..., () => false)`, so a desktop window rendered the
 *    mobile branch on the server and then materialised a 400px column after
 *    hydration — the entire right side of the shell reflowed on every hard load
 *    and every Electron cold start.
 * 2. WHETHER the user collapsed it. This is a cookie, so the server renders the
 *    right box on the first byte, and a module store keeps it live afterwards.
 *
 * The threshold is 1600px, not 1280px. At 1280 a 400px rail left a 544px
 * content column and `/matches` overflowed the viewport by 188px; at 1440 it
 * left 704px and the Library showed two cards. Below the threshold the
 * assistant is a Sheet driven from the command strip, which also retires the
 * floating FAB that used to land on top of the sticky reel action.
 */

import {
  ASSISTANT_RAIL_COOKIE_MAX_AGE,
  ASSISTANT_RAIL_COOKIE_NAME,
} from '@/components/shell/shell-cookies';

/** Below this the assistant is a drawer, not a column. Tailwind reads the same
 *  number as the `--breakpoint-rail` step in globals.css; change both or
 *  neither. */
export const ASSISTANT_WIDE_QUERY = '(min-width: 1600px)';

export type AssistantRailState = 'expanded' | 'collapsed';

/** Absent cookie means expanded: the rail only exists where there is room. */
export function assistantRailStateFromCookie(value: string | undefined): AssistantRailState {
  return value === 'collapsed' ? 'collapsed' : 'expanded';
}

const listeners = new Set<() => void>();
let railState: AssistantRailState | undefined;
let sheetOpen = false;

export function subscribeToAssistantRail(onChange: () => void): () => void {
  listeners.add(onChange);
  return () => {
    listeners.delete(onChange);
  };
}

export function assistantRailSnapshot(): AssistantRailState {
  railState ??= assistantRailStateFromCookie(readCookie(ASSISTANT_RAIL_COOKIE_NAME));
  return railState;
}

export function assistantSheetSnapshot(): boolean {
  return sheetOpen;
}

export function setAssistantRailState(next: AssistantRailState): void {
  if (assistantRailSnapshot() === next) return;
  railState = next;
  document.cookie = `${ASSISTANT_RAIL_COOKIE_NAME}=${next}; path=/; max-age=${ASSISTANT_RAIL_COOKIE_MAX_AGE}; samesite=lax`;
  notify();
}

export function setAssistantSheetOpen(open: boolean): void {
  if (sheetOpen === open) return;
  sheetOpen = open;
  notify();
}

/**
 * The command strip's one assistant control. Which action it performs depends
 * on the layout, and reading that at click time rather than at render time is
 * what keeps the button itself free of a media-query subscription.
 */
export function toggleAssistant(): void {
  if (window.matchMedia(ASSISTANT_WIDE_QUERY).matches) {
    setAssistantRailState(assistantRailSnapshot() === 'collapsed' ? 'expanded' : 'collapsed');
    return;
  }
  setAssistantSheetOpen(!sheetOpen);
}

function notify(): void {
  for (const listener of listeners) listener();
}

function readCookie(name: string): string | undefined {
  const prefix = `${name}=`;
  for (const entry of document.cookie.split('; ')) {
    if (entry.startsWith(prefix)) return entry.slice(prefix.length);
  }
  return undefined;
}
