import { cloudDeviceLinked } from './account-view.ts';
import type { CloudAccountState } from './parse.ts';

/** What the app shell shows: the app, a splash while the account loads, or the sign-in screen. */
export type LoginGate = 'open' | 'checking' | 'sign-in';

/**
 * A capture worker has no cloud client, and a down local service is explained by the app's own screens,
 * so only a known account without this PC linked asks to sign in.
 */
export function loginGate(state: CloudAccountState): LoginGate {
  if (state.kind === 'loading') return 'checking';
  return cloudDeviceLinked(state) ? 'open' : 'sign-in';
}
