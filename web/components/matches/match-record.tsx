'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, MonitorPlay, Smartphone, Swords } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match, Video } from '@/lib/api/types';
import { OUTPUT_STATE, OUTPUT_TYPE, outputState, outputType, type OutputState } from '@/lib/clips/hub';
import { MATCHES_HREF, PRODUCE_FORMAT, produceHref, publishHref } from '@/lib/clips/routes';
import { matchDateLabel, prettyMapName, timeAgo } from '@/lib/format';
import { matchSourceLabel } from '@/lib/records/matches';
import { RecordHeader, type RecordField } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { RelatedList } from '@/components/records/related-list';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { TacticalWorkspace } from '@/components/tactical/tactical-workspace';
import { Button } from '@/components/ui/button';

const VIDEO_STATE: Record<OutputState, { label: string; tone: StatusTagTone }> = {
  [OUTPUT_STATE.queue]: { label: 'En cola', tone: 'neutral' },
  [OUTPUT_STATE.rec]: { label: 'REC', tone: 'stream' },
  [OUTPUT_STATE.render]: { label: 'Edición', tone: 'primary' },
  [OUTPUT_STATE.ready]: { label: 'Listo', tone: 'success' },
  [OUTPUT_STATE.failed]: { label: 'Falló', tone: 'danger' },
};

function matchFields(match: Match | null): RecordField[] {
  if (match === null) return [];
  const fields: RecordField[] = [
    { label: 'Fecha', value: matchDateLabel(match) },
    { label: 'Marcador', value: match.score || 'Sin marcador' },
  ];
  if (match.player) fields.push({ label: 'Jugador', value: match.player });
  const source = matchSourceLabel(match);
  if (source !== '') fields.push({ label: 'Origen', value: source });
  fields.push({ label: 'Jugadas', value: match.decentPlays });
  return fields;
}

/**
 * The record of one partida: its key facts, the tactical analysis and the
 * videos made from it. The match read is independent of the analysis state,
 * so an unanalysed demo is named too.
 */
export function MatchRecord({ matchId }: { matchId: string }): ReactNode {
  const [match, setMatch] = useState<Match | null>(null);
  // `undefined` while loading, `null` when the list could not be read.
  const [videos, setVideos] = useState<Video[] | null | undefined>(undefined);

  useEffect(() => {
    let active = true;
    setMatch(null);
    setVideos(undefined);
    void Promise.allSettled([api.getMatch(matchId), api.listVideos()]).then(([matchResult, videoResult]) => {
      if (!active) return;
      if (matchResult.status === 'fulfilled') setMatch(matchResult.value);
      setVideos(videoResult.status === 'fulfilled' ? videoResult.value.filter((video) => video.jobId === matchId) : null);
    });
    return () => {
      active = false;
    };
  }, [matchId]);

  const map = match?.map ? prettyMapName(match.map) : '';
  useRouteTitle(map ? [map, match?.player].filter(Boolean).join(' · ') : undefined);
  useDocumentTitle(map ? `${map} · Partida · ClipHub` : undefined);

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Partida"
        icon={Swords}
        title={map || 'Partida'}
        fields={matchFields(match)}
        actions={
          <>
            <Button asChild variant="outline" size="sm">
              <Link href={produceHref(matchId, PRODUCE_FORMAT.short)}>
                <Smartphone aria-hidden />
                Crear Short
              </Link>
            </Button>
            <Button asChild variant="outline" size="sm">
              <Link href={produceHref(matchId, PRODUCE_FORMAT.full)}>
                <MonitorPlay aria-hidden />
                Vídeo largo
              </Link>
            </Button>
            <Button asChild variant="ghost" size="sm">
              <Link href={MATCHES_HREF}>
                <ArrowLeft aria-hidden />
                Todas las partidas
              </Link>
            </Button>
          </>
        }
      />
      <RecordTabs
        tabs={[
          { id: 'analisis', label: 'Análisis táctico', content: <TacticalWorkspace jobId={matchId} /> },
          {
            id: 'videos',
            label: 'Vídeos',
            count: videos?.length ?? undefined,
            content: <MatchVideos matchId={matchId} videos={videos} />,
          },
        ]}
      />
    </div>
  );
}

/** The videos made from a partida; each opens its publish page under its own job. */
export function MatchVideos({ matchId, videos }: { matchId: string; videos: readonly Video[] | null | undefined }): ReactNode {
  let body: ReactNode;
  if (videos === undefined) {
    body = <p className="px-4 py-3 text-body-sm text-fg-3">Cargando vídeos</p>;
  } else if (videos === null) {
    body = <p className="px-4 py-3 text-body-sm text-fg-2">No se pudieron leer los vídeos. Comprueba que ClipHub Studio sigue abierto.</p>;
  } else if (videos.length === 0) {
    body = <p className="px-4 py-3 text-body-sm text-fg-2">Aún no has creado ningún vídeo de esta partida.</p>;
  } else {
    body = (
      <ul className="divide-y divide-border-subtle">
        {videos.map((video) => {
          const state = VIDEO_STATE[outputState(video.status)];
          return (
            <li key={video.id}>
              <Link
                href={publishHref(video.jobId ?? matchId, video.id)}
                className="flex min-h-11 flex-wrap items-center gap-x-4 gap-y-1 px-4 py-2 text-body-sm hover:bg-surface-3"
              >
                <span className="min-w-0 flex-1 truncate font-medium text-fg-1">{video.title}</span>
                <span className="text-fg-2">{outputType(video) === OUTPUT_TYPE.full ? 'Vídeo largo' : 'Short'}</span>
                <span className="text-fg-3">{timeAgo(video.createdAt)}</span>
                <StatusTag tone={state.tone} dot>
                  {state.label}
                </StatusTag>
              </Link>
            </li>
          );
        })}
      </ul>
    );
  }
  return (
    <RelatedList title="Vídeos de esta partida" count={videos?.length ?? undefined}>
      {body}
    </RelatedList>
  );
}
