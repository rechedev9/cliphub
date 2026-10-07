/**
 * One in-flight call at a time. A call that arrives while a run is waiting
 * does not start a second request and does not get dropped: the current run
 * is followed by exactly one more, and every caller receives that later
 * result. That is what a delete needs when it lands during a hub poll — the
 * poll must not publish the list it asked for before the delete.
 */
export function createCoalescedCall<T>(run: () => Promise<T>): () => Promise<T> {
  let active: Promise<T> | null = null;
  let again = false;

  function start(): Promise<T> {
    again = false;
    const promise = run().then(
      (value) => (again ? start() : value),
      (error: unknown) => {
        if (again) return start();
        throw error;
      },
    );
    active = promise;
    void promise.finally(() => {
      if (active === promise) active = null;
    });
    return promise;
  }

  return function call(): Promise<T> {
    if (active) {
      again = true;
      return active;
    }
    return start();
  };
}
