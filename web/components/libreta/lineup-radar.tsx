'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { fetchTacticalDocument, type TacticalGeometry, type TacticalSide } from '@/lib/api/tactical';
import { levelForAltitude, worldToRendered } from '@/lib/tactical-transform';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { Skeleton } from '@/components/ui/skeleton';

type Point = [number, number, number];

/**
 * The throw on the radar of the match it was read from: a dot where the
 * thrower stood, a line to where the grenade went off and a ring there.
 */
export function LineupRadar({ matchId, side, origin, landing }: { matchId: string; side: TacticalSide; origin: Point; landing: Point }): ReactNode {
  const [geometry, setGeometry] = useState<TacticalGeometry | null | 'failed'>(null);

  useEffect(() => {
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
      const from = worldToRendered(geometry.calibration, origin[0], origin[1], size);
      const to = worldToRendered(geometry.calibration, landing[0], landing[1], size);
      const scale = size / 1024;
      ctx.lineWidth = Math.max(1.5, 3 * scale);
      ctx.strokeStyle = style.utility;
      ctx.setLineDash([8 * scale, 6 * scale]);
      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      ctx.lineTo(to.x, to.y);
      ctx.stroke();
      ctx.setLineDash([]);
      ctx.beginPath();
      ctx.arc(to.x, to.y, 16 * scale, 0, Math.PI * 2);
      ctx.globalAlpha = 0.3;
      ctx.fillStyle = style.utility;
      ctx.fill();
      ctx.globalAlpha = 1;
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(from.x, from.y, Math.max(4, 8 * scale), 0, Math.PI * 2);
      ctx.fillStyle = side === 'CT' ? style.ct : style.t;
      ctx.fill();
    },
    [geometry, origin, landing, side],
  );
  const marks = useMemo(() => [origin, landing], [origin, landing]);

  if (geometry === 'failed') {
    return <p className="text-body-sm text-fg-2">No se pudo leer el radar de la partida de la que sale este lineup.</p>;
  }
  if (geometry === null) return <Skeleton className="aspect-square w-full max-w-xl" />;
  return (
    <div className="flex flex-col gap-2">
      <RadarSurface
        geometry={geometry}
        level={levelForAltitude(geometry.calibration, origin[2])}
        paint={paint}
        marks={marks}
        label="Radar del lineup: desde dónde se lanza y dónde cae"
      />
      <p className="text-meta text-fg-3">Punto: desde dónde se lanza. Círculo: dónde cae.</p>
    </div>
  );
}
