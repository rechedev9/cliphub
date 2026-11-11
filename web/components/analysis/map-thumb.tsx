'use client';

import { useState, type ReactNode } from 'react';
import type { TacticalGeometry } from '@/lib/api/tactical';
import { mapShotUrl, radarImageUrl } from '@/lib/api/game-assets';
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
 * A map's thumbnail from CS2's own files, darkened so text can sit beside or over it:
 * the loading-screen picture when `photo` asks for it, else the radar. Each missing
 * picture falls back to the next, then to the play-derived silhouette, then to an
 * empty tile of the same size, so rows never shift.
 */
export function MapThumb({
  map,
  geometry,
  className,
  crop = 1.25,
  photo = false,
}: {
  map: string;
  /** Play-derived geometry for the fallback silhouette. */
  geometry?: TacticalGeometry | null;
  /** Size and shape of the tile, such as `size-5` or `h-16 w-full`. */
  className?: string;
  /** How far the radar is zoomed in to cut its empty margin. */
  crop?: number;
  /** Lead with the map's picture; `crop` then applies to the radar fallback only. */
  photo?: boolean;
}): ReactNode {
  const sources = photo ? [mapShotUrl(map), radarImageUrl(map)] : [radarImageUrl(map)];
  // Counted per map, so a reused tile retries the pictures when its map changes.
  const [miss, setMiss] = useState({ map, count: 0 });
  const missing = miss.map === map ? miss.count : 0;
  const source = sources[missing];
  const shot = photo && missing === 0;
  const shape = source === undefined && geometry ? mapSilhouette(geometry, 100) : null;
  return (
    <span aria-hidden className={cn('relative inline-block shrink-0 overflow-hidden rounded-sm bg-surface-0', className)}>
      {shape ? (
        <svg viewBox={shape.viewBox} className="absolute inset-0 size-full text-fg-4">
          <path d={shape.path} fill="currentColor" />
        </svg>
      ) : null}
      {source === undefined ? null : (
        // A plain img: the picture is served by the local API, not a static asset Next could optimise.
        <img
          key={source}
          src={source}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setMiss({ map, count: missing + 1 })}
          className={cn('absolute inset-0 size-full object-cover', shot ? 'brightness-[0.85]' : 'brightness-[0.8] saturate-[0.7]')}
          style={shot ? undefined : { transform: `scale(${crop})` }}
        />
      )}
    </span>
  );
}
