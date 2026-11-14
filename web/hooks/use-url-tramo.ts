'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { tramoFromQuery, writeTramo, type Tramo } from '@/lib/replay/tramo';
import type { TramoControl } from '@/components/replay/tramo-bar';

/** How long a drag rests before the URL catches up. */
const URL_WRITE_DELAY_MS = 150;

function tramoKey(tramo: Tramo): string {
  return tramo.fromPlant === true ? 'plant' : `${tramo.from}-${tramo.to}`;
}

/**
 * The tramo kept in the URL, moved in React state first. A router navigation per pointer move asked the
 * server for the page each time, so the web edition dragged a beat behind; the URL now follows through
 * history.replaceState, which Next syncs into useSearchParams without a server request.
 */
export function useUrlTramo(): TramoControl {
  const search = useSearchParams();
  const fromUrl = useMemo(() => tramoFromQuery(search), [search]);
  const [tramo, setTramo] = useState(fromUrl);
  const [urlKey, setUrlKey] = useState(() => tramoKey(fromUrl));
  const written = useRef<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // A back button or a link changes the URL under us: adopt it, but not the echo of our own write.
  const key = tramoKey(fromUrl);
  if (key !== urlKey) {
    setUrlKey(key);
    if (key !== written.current) setTramo(fromUrl);
  }

  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
    },
    [],
  );

  const onChange = useCallback((next: Tramo) => {
    setTramo(next);
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      timer.current = null;
      written.current = tramoKey(next);
      const query = writeTramo(window.location.search.slice(1), next);
      window.history.replaceState(null, '', query === '' ? window.location.pathname : `${window.location.pathname}?${query}`);
    }, URL_WRITE_DELAY_MS);
  }, []);

  return useMemo(() => ({ tramo, onChange }), [tramo, onChange]);
}
