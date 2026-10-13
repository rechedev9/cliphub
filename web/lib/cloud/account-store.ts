import { cloudApi } from '../api/cloud.ts';
import { startPollLoop } from '../poll-loop.ts';
import type { CloudAccountState } from './parse.ts';

/** One shared copy of the cloud account for every component that shows it. */

/** The orchestrator refreshes its own cache every 20 s; a pending link is watched closely. */
const LINKING_POLL_MS = 2000;
const IDLE_POLL_MS = 20000;

const LOADING: CloudAccountState = { kind: 'loading' };

let current: CloudAccountState = LOADING;
let stopLoop: (() => void) | null = null;
const listeners = new Set<() => void>();

function linkPending(state: CloudAccountState): boolean {
  return state.kind === 'ready' && state.account.link?.status === 'pending';
}

/** Publishes a state; an identical document keeps the old reference so nothing re-renders. */
export function publishCloudAccount(next: CloudAccountState): void {
  if (JSON.stringify(next) === JSON.stringify(current)) return;
  current = next;
  for (const listener of listeners) listener();
}

/** Reads the account now; call it after any mutation that changes it. */
export async function refreshCloudAccount(): Promise<CloudAccountState> {
  publishCloudAccount(await cloudApi.account());
  return current;
}

/** Polls only while some component is subscribed. */
export function subscribeToCloudAccount(listener: () => void): () => void {
  listeners.add(listener);
  if (stopLoop === null) {
    stopLoop = startPollLoop({
      tick: async () => (linkPending(await refreshCloudAccount()) ? 'fast' : 'idle'),
      fastMs: LINKING_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && stopLoop !== null) {
      stopLoop();
      stopLoop = null;
    }
  };
}

export function cloudAccountSnapshot(): CloudAccountState {
  return current;
}

/** SSR snapshot: the server knows no account. */
export function serverCloudAccountSnapshot(): CloudAccountState {
  return LOADING;
}
