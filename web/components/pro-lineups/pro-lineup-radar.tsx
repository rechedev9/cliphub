'use client';

import { useCallback, type ReactNode } from 'react';
import type { ProLineup, ProLineupKind } from '@/lib/api/pro-lineups';
import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import { worldToRendered } from '@/lib/tactical-transform';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';

/** How far from a landing, in world units, a click still picks it. */
const PICK_REACH = 140;

type Props = {
  geometry: TacticalGeometry;
  level: RadarLevel;
  lineups: readonly ProLineup[];
  /** The lineup drawn with its throw line; the rest are landing dots. */
  focus: ProLineup | null;
  onPick?: (lineup: ProLineup) => void;
  label: string;
};

/** The id of the lineup whose landing is nearest a world point, if one is close enough. */
export function nearestLanding(lineups: readonly ProLineup[], point: { x: number; y: number }): ProLineup | null {
  let best: ProLineup | null = null;
  let bestDistance = PICK_REACH;
  for (const lineup of lineups) {
    const d = Math.hypot(lineup.landing[0] - point.x, lineup.landing[1] - point.y);
    if (d < bestDistance) {
      best = lineup;
      bestDistance = d;
    }
  }
  return best;
}

/**
 * Pro lineups on the radar: a dot per landing coloured by grenade, bigger the
 * more it is thrown; the focused lineup also shows where it is thrown from.
 */
export function ProLineupRadar({ geometry, level, lineups, focus, onPick, label }: Props): ReactNode {
  const paint = useCallback(
    ({ ctx, size, style }: RadarFrame) => {
      const computed = getComputedStyle(ctx.canvas);
      const colors = new Map<ProLineupKind, string>();
      const colorOf = (kind: ProLineupKind): string => {
        const known = colors.get(kind);
        if (known !== undefined) return known;
        const value = computed.getPropertyValue(GRENADE_KIND_TOKEN[kind]).trim() || '#9aa4b2';
        colors.set(kind, value);
        return value;
      };
      const scale = size / 1024;
      const top = lineups.reduce((max, l) => Math.max(max, l.throws.all), 1);
      ctx.globalAlpha = focus === null ? 0.85 : 0.35;
      for (const lineup of lineups) {
        if (lineup.id === focus?.id) continue;
        const at = worldToRendered(geometry.calibration, lineup.landing[0], lineup.landing[1], size);
        ctx.beginPath();
        ctx.arc(at.x, at.y, Math.max(2.5, (4 + 6 * Math.sqrt(lineup.throws.all / top)) * scale), 0, Math.PI * 2);
        ctx.fillStyle = colorOf(lineup.kind);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      if (focus === null) return;
      const from = worldToRendered(geometry.calibration, focus.origin[0], focus.origin[1], size);
      const to = worldToRendered(geometry.calibration, focus.landing[0], focus.landing[1], size);
      const color = colorOf(focus.kind);
      ctx.lineWidth = Math.max(1.5, 3 * scale);
      ctx.strokeStyle = color;
      ctx.setLineDash([8 * scale, 6 * scale]);
      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      ctx.lineTo(to.x, to.y);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.beginPath();
      ctx.arc(to.x, to.y, 16 * scale, 0, Math.PI * 2);
      ctx.globalAlpha = 0.3;
      ctx.fillStyle = color;
      ctx.fill();
      ctx.globalAlpha = 1;
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(from.x, from.y, Math.max(4, 8 * scale), 0, Math.PI * 2);
      ctx.fillStyle = focus.side === 'CT' ? style.ct : style.t;
      ctx.fill();
    },
    [geometry.calibration, lineups, focus],
  );

  const onPoint = useCallback(
    (world: { x: number; y: number }) => {
      if (onPick === undefined) return;
      const hit = nearestLanding(lineups, world);
      if (hit !== null) onPick(hit);
    },
    [lineups, onPick],
  );

  return <RadarSurface geometry={geometry} level={level} paint={paint} onPoint={onPick === undefined ? undefined : onPoint} label={label} />;
}
