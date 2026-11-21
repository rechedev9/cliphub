export type SlotPool = { request: (onGrant: () => void) => () => void };

export function createSlotPool(limit: number): SlotPool {
  let busy = 0;
  const waiting: { grant: () => void }[] = [];
  const grantNext = () => {
    const next = waiting.shift();
    if (next) next.grant();
  };
  return {
    request(onGrant) {
      let state: 'waiting' | 'granted' | 'done' = 'waiting';
      const entry = {
        grant: () => {
          state = 'granted';
          busy += 1;
          onGrant();
        },
      };
      if (busy < limit) entry.grant();
      else waiting.push(entry);
      return () => {
        if (state === 'waiting') waiting.splice(waiting.indexOf(entry), 1);
        if (state === 'granted') {
          busy -= 1;
          grantNext();
        }
        state = 'done';
      };
    },
  };
}
