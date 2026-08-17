import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { mapPlateKey, resolveMapPlate, type MapPlate } from '@/lib/map-plate';

export type MapCoverProps = {
  map: string;
  className?: string;
};

/**
 * Tiny map still for the Partidas scoreboard. Real cover photos are rare on
 * uploaded demos, so each known map gets its own palette and silhouette instead
 * of the generic horizon plate that made Ancient and Mirage look identical.
 */
export function MapCover({ map, className }: MapCoverProps): ReactNode {
  const plate = resolveMapPlate(map);
  const skyId = `mc-sky-${plate.id === 'unknown' ? mapPlateKey(map) || 'x' : plate.id}`;
  return (
    <div aria-hidden className={cn('relative size-full overflow-hidden bg-surface-1', className)}>
      <svg
        viewBox="0 0 88 50"
        preserveAspectRatio="xMidYMid slice"
        className="absolute inset-0 size-full forced-colors:hidden"
      >
        <defs>
          <linearGradient id={skyId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor={plate.sky} />
            <stop offset="0.58" stopColor={plate.mid} />
            <stop offset="1" stopColor={plate.ground} />
          </linearGradient>
        </defs>
        <rect width="88" height="50" fill={`url(#${skyId})`} />
        <MapGlyph plate={plate} />
        <rect width="88" height="50" fill="var(--surface-0)" opacity="0.18" />
      </svg>
    </div>
  );
}

function MapGlyph({ plate }: { plate: MapPlate }): ReactNode {
  const { id, accent, rim, ground } = plate;
  switch (id) {
    case 'mirage':
      return (
        <g fill={accent}>
          <path d="M8 50 V28 H22 V50 Z" fill={ground} />
          <path d="M22 50 V22 H54 L62 30 V50 Z" />
          <path d="M28 32 H40 V50 H28 Z" fill={plate.sky} />
          <path d="M64 50 V14 H70 V8 H74 V14 H80 V50 Z" />
          <circle cx="72" cy="18" r="2.2" fill={plate.sky} />
        </g>
      );
    case 'inferno':
      return (
        <g>
          <path d="M0 50 L10 34 H24 L34 50 Z" fill={accent} />
          <path d="M30 50 L42 24 L54 50 Z" fill={rim} />
          <path d="M50 50 V28 H78 V50 Z" fill={accent} />
          <path d="M56 36 H72 V50 H56 Z" fill={plate.sky} />
          <path d="M60 28 V16 H68 V28 Z" fill={rim} />
        </g>
      );
    case 'ancient':
      return (
        <g fill={accent}>
          <path d="M10 50 L22 38 H34 L46 50 Z" />
          <path d="M24 38 L34 26 H50 L60 38 Z" fill={rim} />
          <path d="M36 26 L44 16 H56 L64 26 Z" />
          <path d="M6 50 Q18 42 14 32 Q8 28 16 22" fill={plate.mid} />
          <path d="M72 50 Q78 36 84 28 Q80 40 86 50 Z" fill={plate.mid} />
        </g>
      );
    case 'dust2':
      return (
        <g>
          <ellipse cx="18" cy="46" rx="22" ry="8" fill={ground} />
          <ellipse cx="70" cy="47" rx="24" ry="7" fill={ground} />
          <path d="M28 50 V26 H40 V50 Z" fill={accent} />
          <path d="M48 50 V26 H60 V50 Z" fill={accent} />
          <path d="M32 34 H36 V50 H32 Z" fill={plate.sky} />
          <path d="M52 34 H56 V50 H52 Z" fill={plate.sky} />
          <circle cx="70" cy="14" r="6" fill={rim} />
        </g>
      );
    case 'nuke':
      return (
        <g fill={rim}>
          <path d="M14 50 C14 28 22 18 26 18 C30 18 38 28 38 50 Z" />
          <path d="M42 50 C42 28 50 18 54 18 C58 18 66 28 66 50 Z" />
          <rect x="70" y="28" width="12" height="22" fill={accent} />
          <rect x="72" y="32" width="8" height="4" fill={plate.sky} />
          <rect x="72" y="38" width="8" height="4" fill={plate.sky} />
        </g>
      );
    case 'overpass':
      return (
        <g>
          <path d="M0 36 H88 V42 H0 Z" fill={rim} />
          <path d="M8 36 C20 16 36 16 44 36" fill="none" stroke={accent} strokeWidth="3" />
          <path d="M18 50 V36 H24 V50 Z" fill={ground} />
          <path d="M64 50 V36 H70 V50 Z" fill={ground} />
          <path d="M0 42 H88 V50 H0 Z" fill={ground} />
        </g>
      );
    case 'vertigo':
      return (
        <g fill={accent}>
          <rect x="8" y="20" width="18" height="30" />
          <rect x="30" y="8" width="22" height="42" />
          <rect x="56" y="16" width="16" height="34" />
          <rect x="12" y="26" width="10" height="3" fill={plate.sky} />
          <rect x="36" y="14" width="10" height="3" fill={plate.sky} />
          <rect x="36" y="22" width="10" height="3" fill={plate.sky} />
          <rect x="60" y="22" width="8" height="3" fill={plate.sky} />
          <path d="M74 18 H86 V22 H80 V50 H74 Z" fill={rim} />
        </g>
      );
    case 'anubis':
      return (
        <g>
          <path d="M18 44 L44 10 L70 44 Z" fill={accent} />
          <path d="M32 44 L44 26 L56 44 Z" fill={rim} />
          <rect x="0" y="42" width="88" height="8" fill={plate.accent} opacity="0.45" />
        </g>
      );
    case 'train':
      return (
        <g fill={accent}>
          <rect x="6" y="28" width="22" height="16" />
          <rect x="32" y="28" width="22" height="16" />
          <rect x="58" y="28" width="24" height="16" />
          <rect x="10" y="32" width="6" height="5" fill={plate.sky} />
          <rect x="36" y="32" width="6" height="5" fill={plate.sky} />
          <rect x="62" y="32" width="6" height="5" fill={plate.sky} />
          <path d="M0 46 H88 V50 H0 Z" fill={rim} />
        </g>
      );
    case 'office':
      return (
        <g>
          <rect x="10" y="10" width="68" height="40" fill={rim} />
          <rect x="14" y="14" width="60" height="32" fill={plate.mid} />
          <rect x="18" y="18" width="10" height="8" fill={accent} />
          <rect x="32" y="18" width="10" height="8" fill={accent} />
          <rect x="46" y="18" width="10" height="8" fill={accent} />
          <rect x="60" y="18" width="10" height="8" fill={accent} />
          <rect x="18" y="30" width="10" height="8" fill={accent} />
          <rect x="32" y="30" width="10" height="8" fill={accent} />
          <rect x="46" y="30" width="10" height="8" fill={accent} />
          <rect x="60" y="30" width="10" height="8" fill={accent} />
        </g>
      );
    case 'italy':
      return (
        <g fill={accent}>
          <path d="M8 50 L20 32 H34 L46 50 Z" />
          <rect x="50" y="22" width="16" height="28" />
          <path d="M46 22 H70 L58 8 Z" fill={rim} />
          <rect x="56" y="28" width="4" height="8" fill={plate.sky} />
        </g>
      );
    case 'agency':
      return (
        <g>
          <rect x="6" y="18" width="36" height="32" fill={rim} />
          <rect x="46" y="26" width="36" height="24" fill={accent} />
          <rect x="12" y="24" width="8" height="6" fill={plate.sky} />
          <rect x="24" y="24" width="8" height="6" fill={plate.sky} />
          <rect x="52" y="32" width="8" height="6" fill={plate.sky} />
          <rect x="64" y="32" width="8" height="6" fill={plate.sky} />
        </g>
      );
    default:
      return (
        <g fill={accent}>
          <rect x="12" y="24" width="20" height="26" />
          <rect x="36" y="16" width="16" height="34" />
          <rect x="56" y="28" width="20" height="22" />
        </g>
      );
  }
}
