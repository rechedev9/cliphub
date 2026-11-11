'use client';

import { useState } from 'react';

/**
 * The value, with the identity of its last different content: a rebuilt but equal array or object does not
 * rerun the effects that read it, such as a fetch keyed on the scope's query entries.
 */
export function useContentStable<T>(value: T): T {
  const [held, setHeld] = useState(value);
  if (JSON.stringify(held) === JSON.stringify(value)) return held;
  setHeld(value);
  return value;
}
