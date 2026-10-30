'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Clapperboard } from 'lucide-react';
import { fetchTacticalDocument, isServiceUnavailableError, type TacticalDocument } from '@/lib/api/tactical';
import type { LibraryMatch } from '@/lib/api/library';
import { endReasonLabel, teamByKey, teamLabel } from '@/lib/library/partidas';
import { roundEvidenceHref, roundRows } from '@/lib/library/rounds';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { RelatedList } from '@/components/records/related-list';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

function clock(seconds: number): string {
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}

/** Every round of a partida, each one a link into the 2D replay at the moment the round goes live. */
export function MatchRounds({
  match,
  pathname,
  onOpenRound,
}: {
  match: LibraryMatch;
  pathname: string;
  onOpenRound: () => void;
}): ReactNode {
  const [doc, setDoc] = useState<TacticalDocument | null>(null);
  const [error, setError] = useState<string | null>(null);
  /** The rounds the video dialog opens with checked; null while it is closed. */
  const [video, setVideo] = useState<readonly number[] | null>(null);

  useEffect(() => {
    let active = true;
    setDoc(null);
    setError(null);
    fetchTacticalDocument({ kind: 'library', id: match.id })
      .then((next) => {
        if (active) setDoc(next);
      })
      .catch((err: unknown) => {
        if (!active) return;
        setError(
          isServiceUnavailableError(err)
            ? 'El servicio local no está disponible.'
            : 'No se pudo leer el documento táctico de esta partida.',
        );
      });
    return () => {
      active = false;
    };
  }, [match.id]);

  let body: ReactNode;
  if (error !== null) {
    body = <p className="px-4 py-3 text-body-sm text-destructive">{error}</p>;
  } else if (doc === null) {
    body = (
      <div role="status" aria-label="Cargando rondas" className="p-4">
        <Skeleton className="h-48 w-full" />
      </div>
    );
  } else {
    const rows = roundRows(doc);
    body = (
      <div className="overflow-x-auto">
        <table aria-label="Rondas" className="w-full border-collapse text-left text-body-sm">
          <thead className="bg-surface-2">
            <tr className="border-b border-border text-fg-3">
              {['Ronda', 'Marcador', 'Ganador', 'Motivo', 'Compra T', 'Compra CT', 'Patrón T', 'Patrón CT', 'Sitio', 'Duración', 'Vídeo'].map((header) => (
                <th key={header} scope="col" className="h-9 px-3 font-medium whitespace-nowrap">
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const winner = row.winnerTeamKey === 'ct-start' || row.winnerTeamKey === 't-start' ? teamByKey(match, row.winnerTeamKey) : undefined;
              return (
                <tr key={row.number} className="border-b border-border-subtle last:border-b-0 hover:bg-surface-3">
                  <td className="h-10 px-3">
                    <Link
                      href={roundEvidenceHref(pathname, row.number, row.tick)}
                      onClick={onOpenRound}
                      className="font-mono font-medium text-fg-1 hover:text-primary hover:underline"
                      aria-label={`Abrir la ronda ${row.number} en el replay, tick ${row.tick}`}
                    >
                      {row.number}
                    </Link>
                  </td>
                  <td className="px-3 font-mono text-fg-2">{row.scoreBefore}</td>
                  <td className="px-3">
                    {row.winnerSide === '' ? (
                      <span className="text-fg-3">Sin ganador</span>
                    ) : (
                      <span className="flex items-center gap-2">
                        <span className={cn('font-mono text-meta font-semibold', row.winnerSide === 'CT' ? 'text-side-ct' : 'text-side-t')}>
                          {row.winnerSide}
                        </span>
                        <span className="text-fg-1">{teamLabel(winner)}</span>
                      </span>
                    )}
                  </td>
                  <td className="px-3 text-fg-2">{endReasonLabel(row.round.end_reason)}</td>
                  <td className="px-3 text-fg-2">{buyLabel(row.round.economy.t_buy)}</td>
                  <td className="px-3 text-fg-2">{buyLabel(row.round.economy.ct_buy)}</td>
                  <td className="px-3 text-fg-2">{tPatternLabel(row.round.class.t_side)}</td>
                  <td className="px-3 text-fg-2">{ctPatternLabel(row.round.class.ct_side)}</td>
                  <td className="px-3 text-fg-2">{siteLabel(row.round.class.site)}</td>
                  <td className="px-3 font-mono text-fg-2">{clock(row.liveSeconds)}</td>
                  <td className="px-3">
                    <Button type="button" size="xs" variant="ghost" onClick={() => setVideo([row.number])} aria-label={`Crear vídeo de la ronda ${row.number}`}>
                      <Clapperboard aria-hidden />
                      Vídeo
                    </Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    );
  }

  const allRounds = doc?.rounds.map((round) => round.number) ?? [];
  return (
    <>
      <RelatedList
        title="Rondas"
        count={doc?.rounds.length}
        actions={
          doc ? (
            <Button type="button" size="sm" variant="outline" onClick={() => setVideo([])}>
              <Clapperboard aria-hidden />
              Vídeo de varias rondas
            </Button>
          ) : null
        }
      >
        {body}
      </RelatedList>
      {video !== null ? (
        <AnalysisVideoDialog sources={[{ matchId: match.id, rounds: allRounds }]} selected={video} onClose={() => setVideo(null)} />
      ) : null}
    </>
  );
}
