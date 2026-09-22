/**
 * Open/seen state for the Studio tour layer.
 *
 * The tour lives in the app layout, its "Guía" trigger in the command strip,
 * so the two meet through this module rather than through React context. The
 * telemetry notice reports here too: both are first-run dialogs, and the tour
 * must wait until the notice is answered instead of stacking on top of it.
 */

/** localStorage key; bump the version when the tour gains content worth replaying. */
export const TOUR_SEEN_KEY = 'cliphub.tour.v1';

/** `pending` until TelemetryNotice knows whether it will show. */
export type TelemetryNoticeState = 'pending' | 'open' | 'settled';

export interface AppTourState {
  readonly open: boolean;
  /** Chapter index the tour opens on. */
  readonly chapter: number;
  readonly telemetryNotice: TelemetryNoticeState;
}

const INITIAL: AppTourState = { open: false, chapter: 0, telemetryNotice: 'pending' };

let current: AppTourState = INITIAL;
const listeners = new Set<() => void>();

function set(next: AppTourState): void {
  if (
    next.open === current.open &&
    next.chapter === current.chapter &&
    next.telemetryNotice === current.telemetryNotice
  ) {
    return;
  }
  current = next;
  for (const listener of listeners) listener();
}

export function subscribeToAppTour(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function appTourSnapshot(): AppTourState {
  return current;
}

/** SSR snapshot. Constant by construction: the server never opens the tour. */
export function serverAppTourSnapshot(): AppTourState {
  return INITIAL;
}

export function openAppTour(chapter = 0): void {
  set({ ...current, open: true, chapter });
}

export function closeAppTour(): void {
  set({ ...current, open: false });
}

export function setAppTourChapter(chapter: number): void {
  set({ ...current, chapter });
}

export function reportTelemetryNotice(state: TelemetryNoticeState): void {
  set({ ...current, telemetryNotice: state });
}

/** Test seam: back to the first-load state without notifying. */
export function resetAppTour(): void {
  current = INITIAL;
}

/**
 * First-run auto-open. Only inside Studio (`desktop`): a plain browser is
 * frontend development or the e2e contract, where a modal on every first
 * visit would sit on top of whatever the page is meant to show.
 */
export function shouldAutoOpenTour(input: {
  readonly desktop: boolean;
  readonly seen: boolean;
  readonly telemetryNotice: TelemetryNoticeState;
}): boolean {
  return input.desktop && !input.seen && input.telemetryNotice === 'settled';
}

type TourStorage = Pick<Storage, 'getItem' | 'setItem'>;

/**
 * Unreadable storage counts as seen: a private window or blocked site data
 * would otherwise reopen the tour on every launch.
 */
export function tourWasSeen(storage: TourStorage | null): boolean {
  if (storage === null) return true;
  try {
    return storage.getItem(TOUR_SEEN_KEY) !== null;
  } catch {
    return true;
  }
}

export function markTourSeen(storage: TourStorage | null, now: number): void {
  if (storage === null) return;
  try {
    storage.setItem(TOUR_SEEN_KEY, new Date(now).toISOString());
  } catch {
    // Nothing to do: the worst case is seeing the tour once more.
  }
}
