'use client';

import { useState, type ReactNode } from 'react';
import type { TacticalGeometry } from '@/lib/api/tactical';
import { radarImageUrl } from '@/lib/api/game-assets';
import { mapSilhouette } from '@/lib/analysis/map-silhouette';
import { prettyMapName } from '@/lib/format';
import { cn } from '@/lib/utils';

/** A map's name led by its radar thumbnail, for a table cell or a list row. */
export function MapLabel({ map }: { map: string }): ReactNode {
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <MapThumb map={map} className="h-5 w-8" crop={1.5} />
      {prettyMapName(map)}
    </span>
  );
}

/**
 * A map's thumbnail cut from CS2's own radar, darkened so text can sit beside
 * or over it. Without CS2 it falls back to the play-derived silhouette, and
 * without that to an empty tile of the same size, so rows never shift.
 */
export function MapThumb({
  map,
  geometry,
  className,
  crop = 1.25,
}: {
  map: string;
  /** Play-derived geometry for the fallback silhouette. */
  geometry?: TacticalGeometry | null;
  /** Size and shape of the tile, such as `size-5` or `h-16 w-full`. */
  className?: string;
  /** How far the radar is zoomed in to cut its empty margin. */
  crop?: number;
}): ReactNode {
  const [failed, setFailed] = useState(false);
  const shape = failed && geometry ? mapSilhouette(geometry, 100) : null;
  return (
    <span aria-hidden className={cn('relative inline-block shrink-0 overflow-hidden rounded-sm bg-surface-0', className)}>
      {shape ? (
        <svg viewBox={shape.viewBox} className="absolute inset-0 size-full text-fg-4">
          <path d={shape.path} fill="currentColor" />
        </svg>
      ) : null}
      {failed ? null : (
        // A plain img: the radar is served by the local API, not a static asset Next could optimise.
        <img
          src={radarImageUrl(map)}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailed(true)}
          className="absolute inset-0 size-full object-cover brightness-[0.8] saturate-[0.7]"
          style={{ transform: `scale(${crop})` }}
        />
      )}
    </span>
  );
}
