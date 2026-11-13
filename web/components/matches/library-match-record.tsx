'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ArrowLeft, Swords } from 'lucide-react';
import { api } from '@/lib/api';
import type { Video } from '@/lib/api/types';
import type { LibraryDemo, LibraryMatch } from '@/lib/api/library';
import { MATCHES_HREF, librarySeriesHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { currentEdition } from '@/lib/edition';
import { SOURCE_LABEL, matchTitle, scoreline } from '@/lib/library/partidas';
import { LEGACY_REPLAY_TAB, MATCH_REPLAY_TAB, MATCH_SUMMARY_TAB } from '@/lib/library/rounds';
import { useRecordTab } from '@/hooks/use-record-tab';
import { RecordHeader } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { CreateFromDemo } from '@/components/matches/create-from-demo';
import { MatchVideos } from '@/components/matches/match-record';
import { MatchMetrics } from '@/components/metrics/match-metrics';
import { MatchRounds } from '@/components/matches/match-rounds';
import { MatchSummary } from '@/components/matches/match-summary';
import { TacticalAnalysis } from '@/components/tactical/tactical-analysis';
import { MatchTendencies } from '@/components/metrics/match-tendencies';
import { Button } from '@/components/ui/button';

const TABS = [MATCH_SUMMARY_TAB, 'rondas', MATCH_REPLAY_TAB, 'metricas', 'videos'] as const;
type MatchTab = (typeof TABS)[number];
const QUERY_TABS = [...TABS, LEGACY_REPLAY_TAB] as const;

/** Videos are made on the PC, so the web edition has no Vídeos tab. */
function tabFromQuery(raw: string | null, canVideo: boolean): MatchTab | null {
  if (raw === LEGACY_REPLAY_TAB) return MATCH_REPLAY_TAB;
  if (raw === 'videos' && !canVideo) return null;
  return TABS.find((tab) => tab === raw) ?? null;
}

/** Minutes of play, or null when the demo carries no tickrate to count them with. */
function durationLabel(match: LibraryMatch): string | null {
  if (!(match.tickrate > 0)) return null;
  return `${Math.round(match.duration_ticks / match.tickrate / 60)} min`;
}

/** The score, date, duration, rounds, source and series as one line under the map. */
function matchMeta(match: LibraryMatch, demo: LibraryDemo): ReactNode {
  const [left, right] = scoreline(match);
  const parts = [
    `${left.name} ${left.score} : ${right.score} ${right.name}`,
    formatShortDate(match.played_at),
    durationLabel(match),
    `${match.round_count} ${match.round_count === 1 ? 'ronda' : 'rondas'}`,
    SOURCE_LABEL[demo.source],
  ].filter((part) => part !== null);
  return (
    <>
      {parts.join(' · ')}
      {' · '}
      {match.series_id ? (
        <Link href={librarySeriesHref(match.series_id)} className="text-fg-2 underline decoration-fg-4 underline-offset-2 hover:text-fg-1">
          Ver serie
        </Link>
      ) : (
        'Sin serie'
      )}
    </>
  );
}

/**
 * The record of one library partida: the teams and their scoreboards, every
 * round with a link into the replay, the Replay, and the videos
 * Crear made from the same demo.
 */
export function LibraryMatchRecord({ match: loadedMatch, demo: loadedDemo }: { match: LibraryMatch; demo: LibraryDemo }): ReactNode {
  const pathname = usePathname();
  const [match, setMatch] = useState(loadedMatch);
  const [demo, setDemo] = useState(loadedDemo);
  // The open tab lives in `?tab=`, so a reload or an evidence link lands on it.
  const [tabValue, setTab] = useRecordTab(QUERY_TABS, MATCH_SUMMARY_TAB);
  const canVideo = currentEdition() === 'desktop';
  const tab = tabFromQuery(tabValue, canVideo) ?? MATCH_SUMMARY_TAB;
  // `undefined` while loading, `null` when the list could not be read.
  const [videos, setVideos] = useState<Video[] | null | undefined>(undefined);
  const jobKey = demo.job_ids.join(',');

  useEffect(() => {
    if (!canVideo) return undefined;
    let active = true;
    const jobs = new Set(jobKey === '' ? [] : jobKey.split(','));
    api
      .listVideos()
      .then((list) => {
        if (active) setVideos(list.filter((video) => video.jobId !== undefined && jobs.has(video.jobId)));
      })
      .catch(() => {
        if (active) setVideos(null);
      });
    return () => {
      active = false;
    };
  }, [jobKey, canVideo]);

  const map = prettyMapName(match.map);
  useRouteTitle(`${map} · ${matchTitle(match)}`);
  useDocumentTitle(`${map} · Partida · ClipHub`);
  const openReplay = (): void => setTab(MATCH_REPLAY_TAB);

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Partida"
        icon={Swords}
        title={map}
        subtitle={matchMeta(match, demo)}
        actions={
          <>
            <CreateFromDemo demo={demo} />
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
        value={tab}
        onValueChange={setTab}
        tabs={[
          {
            id: MATCH_SUMMARY_TAB,
            label: 'Resumen',
            content: (
              <MatchSummary
                match={match}
                demo={demo}
                pathname={pathname}
                onMatchChange={setMatch}
                onDemoChange={setDemo}
                onOpenReplay={openReplay}
              />
            ),
          },
          {
            id: 'rondas',
            label: 'Rondas',
            count: match.round_count,
            content: <MatchRounds match={match} pathname={pathname} onOpenRound={openReplay} />,
          },
          {
            id: MATCH_REPLAY_TAB,
            label: 'Replay',
            content: <TacticalAnalysis source={{ kind: 'library', id: match.id }} generatedAt={match.updated_at} match={match} />,
          },
          {
            id: 'metricas',
            label: 'Métricas',
            content: (
              <div className="flex flex-col gap-6">
                <MatchMetrics match={match} onOpenReplay={openReplay} />
                <MatchTendencies matchId={match.id} />
              </div>
            ),
          },
          ...(canVideo
            ? [
                {
                  id: 'videos',
                  label: 'Vídeos',
                  count: videos?.length ?? undefined,
                  content: (
                    <MatchVideos
                      matchId={demo.job_ids.at(-1) ?? match.id}
                      videos={videos}
                      onVideosChange={setVideos}
                      demo={demo}
                      onOpenRounds={() => setTab('rondas')}
                    />
                  ),
                },
              ]
            : []),
        ]}
      />
    </div>
  );
}
