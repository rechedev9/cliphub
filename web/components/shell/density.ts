'use client';

import { useSyncExternalStore } from 'react';
import { DENSITY_STORAGE_KEY, parseDensity, type Density } from '@/lib/density';

const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function readDensity(): Density {
  return parseDensity(document.documentElement.dataset.density);
}

export function setDensity(density: Density): void {
  const root = document.documentElement;
  if (density === 'compact') root.dataset.density = 'compact';
  else delete root.dataset.density;
  try {
    localStorage.setItem(DENSITY_STORAGE_KEY, density);
  } catch {
    // A blocked store only loses the choice on the next launch.
  }
  for (const listener of listeners) listener();
}

/** The current density, or null before hydration (the server cannot know it). */
export function useDensity(): Density | null {
  return useSyncExternalStore(subscribe, readDensity, () => null);
}
