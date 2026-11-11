'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { BookmarkPlus, Play } from 'lucide-react';
import { detectTeamPlays } from '@/lib/api/library';
import type { DetectedPlay, PlayDetection } from '@/lib/api/stratbook';
import { evidenceRefs } from '@/lib/analysis/playbook';
import { replayHref } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { rateText } from '@/lib/reports/scout-sections';
import { playShapeLabel } from '@/lib/stratbook/plays';
import { SaveDetectedDialog } from '@/components/stratbook/play-detection';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

type Read = { kind: 'loading' } | { kind: 'ready'; detection: PlayDetection } | { kind: 'failed' };

/**
 * The plays the team repeats on this map and side, found by the same grouping as the Libreta's
 * "Jugadas repetidas": each one can be saved as a play or opened as its rounds.
 */
export function DetectedPlays({
  teamId,
  teamName,
  scope,
  lineup,
}: {
  teamId: string;
  teamName: string;
  scope: AnalysisScope;
  lineup: readonly string[];
}): ReactNode {
  const [read, setRead] = useState<Read>({ kind: 'loading' });
  const [saving, setSaving] = useState<DetectedPlay | null>(null);
  const [now] = useState(() => Date.now());
  const { map, side, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);

  useEffect(() => {
    let active = true;
    setRead({ kind: 'loading' });
    detectTeamPlays(teamId, { maps: [map], side, from, to, opponent_ids: opponent === '' ? [] : [opponent], lineup })
      .then((detection) => {
        if (active) setRead({ kind: 'ready', detection });
      })
      .catch(() => {
        if (active) setRead({ kind: 'failed' });
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, from, to, opponent, lineup]);

  if (read.kind === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron buscar las jugadas repetidas. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (read.kind === 'loading') {
    return (
      <div role="status" aria-label="Buscando jugadas repetidas" className="flex flex-col gap-2">
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }
  const { detection } = read;
  if (detection.plays.length === 0) {
    return <p className="text-body-sm text-fg-2">No hay jugadas repetidas en este mapa y lado con las reglas de la Libreta.</p>;
  }
  return (
    <div className="flex flex-col gap-2">
      <p className="text-meta text-fg-3">
        {detection.plays.length} {detection.plays.length === 1 ? 'jugada repetida' : 'jugadas repetidas'} en {detection.rounds} rondas.
      </p>
      <ul className="flex flex-col gap-2">
        {detection.plays.map((play) => (
          <li key={`${play.seed.match_id}:${play.seed.round}`} className="flex flex-col gap-1.5 rounded-sm border border-border bg-surface-2 p-2">
            <span className="text-body-sm font-semibold text-fg-1">{playShapeLabel(play)}</span>
            <span className="text-meta text-fg-2 tabular-nums">
              {play.rounds} rondas, gana {rateText(play.win_rate)}
            </span>
            <span className="flex flex-wrap gap-1.5">
              <Button type="button" size="xs" variant="outline" onClick={() => setSaving(play)}>
                <BookmarkPlus aria-hidden />
                Guardar
              </Button>
              <Button asChild size="xs" variant="ghost">
                <Link href={replayHref(evidenceRefs(play.refs ?? play.examples), `${playShapeLabel(play)} de ${teamName}`)}>
                  <Play aria-hidden />
                  Ver rondas
                </Link>
              </Button>
            </span>
          </li>
        ))}
      </ul>
      {saving === null ? null : (
        <SaveDetectedDialog
          play={saving}
          teamId={teamId}
          teamName={teamName}
          positionSeconds={detection.thresholds.position_seconds}
          onClose={() => setSaving(null)}
          onSaved={() => undefined}
        />
      )}
    </div>
  );
}
