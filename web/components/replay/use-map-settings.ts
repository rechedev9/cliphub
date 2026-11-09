'use client';

import { useCallback, useEffect, useState } from 'react';
import { DEFAULT_MAP_SETTINGS, MAP_SETTINGS_KEY, parseMapSettings } from '@/lib/replay/map-settings';
import type { MapSettings } from '@/lib/replay/map-settings';

/** The map's settings, remembered on this PC; they load after the first paint. */
export function useMapSettings(): [MapSettings, (settings: MapSettings) => void] {
  const [settings, setSettings] = useState<MapSettings>(DEFAULT_MAP_SETTINGS);
  useEffect(() => {
    try {
      setSettings(parseMapSettings(window.localStorage.getItem(MAP_SETTINGS_KEY)));
    } catch {
      // Storage is blocked: the defaults stay.
    }
  }, []);
  const change = useCallback((next: MapSettings) => {
    setSettings(next);
    try {
      window.localStorage.setItem(MAP_SETTINGS_KEY, JSON.stringify(next));
    } catch {
      // Storage is blocked: the choice lasts for this visit.
    }
  }, []);
  return [settings, change];
}
