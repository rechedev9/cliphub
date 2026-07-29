import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

function isServiceUnavailableRejection(
  result: PromiseSettledResult<void>,
): result is PromiseRejectedResult {
  if (result.status !== 'rejected') return false;
  const reason = result.reason;
  return reason instanceof Error
    && 'code' in reason
    && reason.code === SERVICE_UNAVAILABLE_CODE;
}

/**
 * Settles one Library refresh without allowing a reel-local failure to hide
 * successful or cached reels. An all-503 batch is different: every active reel
 * independently observed the stable offline contract, so the page must still
 * surface the global service outage.
 */
export async function reconcileReels(operations: readonly Promise<void>[]): Promise<void> {
  const results = await Promise.allSettled(operations);
  const unavailable = results.find(isServiceUnavailableRejection);
  if (unavailable && results.every(isServiceUnavailableRejection)) {
    throw unavailable.reason;
  }
}
