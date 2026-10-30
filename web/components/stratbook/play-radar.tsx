'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { fetchTacticalDocument, RADAR_LEVELS, type RadarLevel, type TacticalGeometry } from '@/lib/api/tactical';
import type { PlayGrenade, PlayGrenadeKind, PlayPosition } from '@/lib/api/stratbook';
import { GRENADE_LABEL } from '@/lib/stratbook/plays';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, levelForAltitude, worldToRendered } from '@/lib/tactical-transform';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';
import { Skeleton } from '@/components/ui/skeleton';

/** Grenade ring radius in radar pixels at the 1024 reference size. */
const GRENADE_RADIUS: Record<PlayGrenadeKind, number> = { smoke: 18, molotov: 14, flash: 9, he: 9 };

/**
 * A play drawn on the radar of one of its example matches: each grenade where
 * it landed, each player where they stood. The plate is `aria-hidden` through
 * RadarSurface; the record lists the same grenades and positions as tables.
 */
export function PlayRadar({
  matchId,
  side,
  grenades,
  positions,
}: {
  /** An example match on the play's map: its document carries the radar. */
  matchId: string | undefined;
  side: 'CT' | 'T';
  grenades: readonly PlayGrenade[];
  positions: readonly PlayPosition[];
}): ReactNode {
  const [geometry, setGeometry] = useState<TacticalGeometry | null | 'failed'>(null);
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);

  useEffect(() => {
    if (matchId === undefined) return;
    let active = true;
    setGeometry(null);
    fetchTacticalDocument({ kind: 'library', id: matchId })
      .then((doc) => {
        if (active) setGeometry(doc.geometry);
      })
      .catch(() => {
        if (active) setGeometry('failed');
      });
    return () => {
      active = false;
    };
  }, [matchId]);

  const paint = useCallback(
    ({ ctx, size, style }: RadarFrame) => {
      if (geometry === null || geometry === 'failed') return;
      const calibration = geometry.calibration;
      const scale = size / 1024;
      for (const grenade of grenades) {
        if (levelForAltitude(calibration, grenade.pos[2]) !== level) continue;
        const p = worldToRendered(calibration, grenade.pos[0], grenade.pos[1], size);
        const radius = GRENADE_RADIUS[grenade.kind] * scale;
        ctx.beginPath();
        ctx.arc(p.x, p.y, radius, 0, Math.PI * 2);
        ctx.globalAlpha = grenade.kind === 'smoke' ? 0.35 : 0.25;
        ctx.fillStyle = grenade.kind === 'molotov' || grenade.kind === 'he' ? style.bomb : style.utility;
        ctx.fill();
        ctx.globalAlpha = 1;
        ctx.lineWidth = Math.max(1, 2 * scale);
        ctx.strokeStyle = ctx.fillStyle;
        ctx.stroke();
      }
      const color = side === 'CT' ? style.ct : style.t;
      ctx.font = `${Math.max(10, 12 * scale)}px ${style.fontFamily}`;
      ctx.textBaseline = 'middle';
      for (const position of positions) {
        if (levelForAltitude(calibration, position.pos[2]) !== level) continue;
        const p = worldToRendered(calibration, position.pos[0], position.pos[1], size);
        const dot = Math.max(3, 7 * scale);
        ctx.beginPath();
        ctx.arc(p.x, p.y, dot, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.fill();
        if (position.player) {
          ctx.fillStyle = style.text;
          ctx.fillText(position.player.slice(0, 12), p.x + dot * 1.6, p.y);
        }
      }
    },
    [geometry, grenades, positions, side, level],
  );

  if (matchId === undefined) {
    return <p className="text-body-sm text-fg-2">Esta jugada no tiene ninguna ronda de ejemplo en la biblioteca, así que no hay radar sobre el que dibujarla.</p>;
  }
  if (geometry === 'failed') {
    return <p role="alert" className="text-body-sm text-destructive">No se pudo leer el radar de la partida de ejemplo.</p>;
  }
  if (geometry === null) return <Skeleton className="aspect-square w-full max-w-xl" />;

  const multi = isMultiLevelMap(geometry.calibration);
  const kinds = [...new Set(grenades.map((grenade) => grenade.kind))];
  return (
    <div className="flex flex-col gap-3">
      {multi ? (
        <Segmented
          label="Piso"
          value={level}
          options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))}
          onChange={setLevel}
        />
      ) : null}
      <RadarSurface
        geometry={geometry}
        level={level}
        paint={paint}
        label={`Radar de la jugada: ${grenades.length} granadas y ${positions.length} posiciones`}
      />
      <p className="text-meta text-fg-3">
        Puntos: posiciones de los jugadores. Círculos: dónde cayó cada granada
        {kinds.length > 0 ? ` (${kinds.map((kind) => GRENADE_LABEL[kind]).join(', ')})` : ''}.
      </p>
    </div>
  );
}
