'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useSearchParams } from 'next/navigation';
import { ArrowLeft, Swords } from 'lucide-react';
import { api } from '@/lib/api';
import type { Video } from '@/lib/api/types';
import type { LibraryDemo, LibraryMatch } from '@/lib/api/library';
import { MATCHES_HREF, librarySeriesHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { SOURCE_LABEL, matchScoreLabel, matchTitle } from '@/lib/library/partidas';
import { MATCH_TAB_QUERY, MATCH_TACTICAL_TAB } from '@/lib/library/rounds';
import { RecordHeader, type RecordField } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { CreateFromDemo } from '@/components/matches/create-from-demo';
import { MatchVideos } from '@/components/matches/match-record';
import { MatchMetrics } from '@/components/metrics/match-metrics';
import { MatchRounds } from '@/components/matches/match-rounds';
import { MatchSummary } from '@/components/matches/match-summary';
import { TacticalAnalysis } from '@/components/tactical/tactical-analysis';
import { Button } from '@/components/ui/button';

const TABS = ['resumen', 'rondas', MATCH_TACTICAL_TAB, 'metricas', 'videos'] as const;
type MatchTab = (typeof TABS)[number];

function tabFromQuery(raw: string | null): MatchTab | null {
  return TABS.find((tab) => tab === raw) ?? null;
}

function durationLabel(match: LibraryMatch): string {
  if (!(match.tickrate > 0)) return 'Sin dato';
  const minutes = Math.round(match.duration_ticks / match.tickrate / 60);
  return `${minutes} min`;
}

function matchFields(match: LibraryMatch, demo: LibraryDemo): RecordField[] {
  return [
    { label: 'Fecha', value: formatShortDate(match.played_at) },
    { label: 'Marcador', value: <span className="font-mono">{matchScoreLabel(match)}</span> },
    { label: 'Rondas', value: match.round_count },
    { label: 'Duración', value: durationLabel(match) },
    { label: 'Fuente', value: SOURCE_LABEL[demo.source] },
    { label: 'Build de CS2', value: demo.cs2_build ?? 'Sin dato' },
    {
      label: 'Serie',
      value: match.series_id ? (
        <Link href={librarySeriesHref(match.series_id)} className="text-fg-1 hover:text-primary hover:underline">
          Ver serie
        </Link>
      ) : (
        'Sin serie'
      ),
    },
  ];
}

/**
 * The record of one library partida: the teams and their scoreboards, every
 * round with a link into the replay, the 2D tactical analysis, and the videos
 * Crear made from the same demo.
 */
export function LibraryMatchRecord({ match: loadedMatch, demo: loadedDemo }: { match: LibraryMatch; demo: LibraryDemo }): ReactNode {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [match, setMatch] = useState(loadedMatch);
  const [demo, setDemo] = useState(loadedDemo);
  const queryTab = tabFromQuery(searchParams.get(MATCH_TAB_QUERY));
  const [tab, setTab] = useState<MatchTab>(queryTab ?? 'resumen');
  // `undefined` while loading, `null` when the list could not be read.
  const [videos, setVideos] = useState<Video[] | null | undefined>(undefined);
  const jobKey = demo.job_ids.join(',');

  // An evidence link on this page names its tab in the URL.
  useEffect(() => {
    if (queryTab !== null) setTab(queryTab);
  }, [queryTab]);

  useEffect(() => {
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
  }, [jobKey]);

  const map = prettyMapName(match.map);
  useRouteTitle(`${map} · ${matchTitle(match)}`);
  useDocumentTitle(`${map} · Partida · ClipHub`);
  const openTactical = (): void => setTab(MATCH_TACTICAL_TAB);

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Partida"
        icon={Swords}
        title={map}
        subtitle={matchTitle(match)}
        fields={matchFields(match, demo)}
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
        onValueChange={(next) => setTab(tabFromQuery(next) ?? 'resumen')}
        tabs={[
          {
            id: 'resumen',
            label: 'Resumen',
            content: (
              <MatchSummary
                match={match}
                demo={demo}
                pathname={pathname}
                onMatchChange={setMatch}
                onDemoChange={setDemo}
                onOpenTactical={openTactical}
              />
            ),
          },
          {
            id: 'rondas',
            label: 'Rondas',
            count: match.round_count,
            content: <MatchRounds match={match} pathname={pathname} onOpenRound={openTactical} />,
          },
          {
            id: MATCH_TACTICAL_TAB,
            label: 'Táctica',
            content: <TacticalAnalysis source={{ kind: 'library', id: match.id }} generatedAt={match.updated_at} />,
          },
          {
            id: 'metricas',
            label: 'Métricas',
            content: <MatchMetrics match={match} />,
          },
          {
            id: 'videos',
            label: 'Vídeos',
            count: videos?.length ?? undefined,
            content: <MatchVideos matchId={demo.job_ids.at(-1) ?? match.id} videos={videos} />,
          },
        ]}
      />
    </div>
  );
}
