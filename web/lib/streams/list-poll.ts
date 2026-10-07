/**
 * Reads a stream list for the poll that is still mounted.
 *
 * The page restarts its poll when a row is deleted. A list request that
 * started before that restart can resolve afterwards; publishing it puts the
 * deleted row back on screen. Callers drop a null result.
 */
export async function readIfCurrent<T>(load: () => Promise<T>, current: () => boolean): Promise<T | null> {
  const value = await load();
  return current() ? value : null;
}
