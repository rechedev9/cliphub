'use client';

import { useEffect, useSyncExternalStore } from 'react';
import { CALLOUT_NAMES, type CalloutLabel } from '@/lib/libreta/callouts';

/**
 * `calloutLabel(map, token)`: the team's name for a CS2 place, else CS2's.
 * Reads each map's names once per session and re-renders when one arrives or changes.
 */
export function useCalloutNames(maps: readonly string[]): CalloutLabel {
  const calloutLabel = useSyncExternalStore(CALLOUT_NAMES.subscribe, CALLOUT_NAMES.label, CALLOUT_NAMES.label);
  const key = [...new Set(maps)].filter((map) => map !== '').sort().join(',');
  useEffect(() => {
    for (const map of key === '' ? [] : key.split(',')) void CALLOUT_NAMES.load(map);
  }, [key, calloutLabel]);
  return calloutLabel;
}
