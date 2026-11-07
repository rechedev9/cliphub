'use client';

import { useCallback, useEffect, useId, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowLeft, Layers, MonitorPlay, Smartphone, Swords, Users, type LucideIcon } from 'lucide-react';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import type { LibraryDemo } from '@/lib/api/library';
import { demoPlaybackItem } from '@/lib/api/playback';
import type { Match, Video } from '@/lib/api/types';
import { OUTPUT_STATE, OUTPUT_TYPE, isHighlightsVideo, isWorking, outputLabel, outputState, outputType, toOutput, type OutputState } from '@/lib/clips/hub';
import { MATCHES_HREF, PRODUCE_FORMAT, produceHref, publishHref, type ProduceFormat } from '@/lib/clips/routes';
import { matchDateLabel, prettyMapName, timeAgo } from '@/lib/format';
import { SHARED_MATCH_REASON } from '@/lib/library/partidas';
import { startPollLoop } from '@/lib/poll-loop';
import { matchSourceLabel } from '@/lib/records/matches';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { OutputActions } from '@/components/clips-hub/output-item';
import { OpenInCrearButton } from '@/components/matches/open-in-crear-button';
import { RecordHeader, type RecordField } from '@/components/records/record-header';
import { RecordTabs } from '@/components/records/record-tabs';
import { RelatedList } from '@/components/records/related-list';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { CoverImage } from '@/components/studio/cover-image';
import { MediaPlayer } from '@/components/studio/media-player';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { TacticalWorkspace } from '@/components/tactical/tactical-workspace';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

const VIDEO_STATE: Record<OutputState, { label: string; tone: StatusTagTone }> = {
  [OUTPUT_STATE.queue]: { label: 'En cola', tone: 'neutral' },
  [OUTPUT_STATE.rec]: { label: 'REC', tone: 'stream' },
  [OUTPUT_STATE.render]: { label: 'Edición', tone: 'primary' },
  [OUTPUT_STATE.ready]: { label: 'Listo', tone: 'success' },
  [OUTPUT_STATE.failed]: { label: 'Falló', tone: 'danger' },
};

/** The kinds of video a partida yields, as Mis vídeos names them. */
type VideoKind = 'short' | 'summary' | 'full';
const VIDEO_KINDS: readonly VideoKind[] = ['short', 'summary', 'full'];
const KIND_CHIP: Record<VideoKind, string> = { short: 'Shorts', summary: 'Resúmenes', full: 'Demos completas' };
// Radix refuses an empty item value, so "every kind" travels as this sentinel inside the chips only.
const ALL_KINDS = '__all__';

const VIDEO_POLL_MS = 3_000;
const VIDEO_IDLE_MS = 15_000;

function videoKind(video: Video): VideoKind {
  if (outputType(video) === OUTPUT_TYPE.short) return 'short';
  return isHighlightsVideo(video) ? 'summary' : 'full';
}

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
                Demo completa
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
            content: <MatchVideos matchId={matchId} videos={videos} onVideosChange={setVideos} />,
          },
        ]}
      />
    </div>
  );
}

export type MatchVideosProps = {
  /** The Crear job the partida's links fall back to. */
  matchId: string;
  /** `undefined` while loading, `null` when the list could not be read. */
  videos: readonly Video[] | null | undefined;
  /** Takes the list a retry, a delete or a poll tick re-read, so the parent's tab count follows it. */
  onVideosChange: (videos: Video[]) => void;
  /** The library demo: with it, the ways to make a video open Crear on a job of their own, even before the first. */
  demo?: LibraryDemo | undefined;
  /** Opens the Rondas tab, where the rounds of a video are picked; without it that path is not offered. */
  onOpenRounds?: (() => void) | undefined;
};

/** The videos made from a partida, or the ways to make the first one. */
export function MatchVideos({ matchId, videos, onVideosChange, demo, onOpenRounds }: MatchVideosProps): ReactNode {
  const loaded = videos !== undefined && videos !== null;
  const jobKey = [matchId, ...(demo?.job_ids ?? []), ...(videos ?? []).flatMap((video) => (video.jobId ? [video.jobId] : []))].join(',');

  const refresh = useCallback(async () => {
    if (!loaded) return;
    const jobs = new Set(jobKey.split(','));
    const list = await api.listVideos();
    onVideosChange(list.filter((video) => video.jobId !== undefined && jobs.has(video.jobId)));
  }, [loaded, jobKey, onVideosChange]);

  const working = videos?.some((video) => isWorking(outputState(video.status))) ?? false;
  useEffect(() => {
    if (!working) return undefined;
    return startPollLoop({
      tick: async () => {
        await refresh();
        return 'fast';
      },
      fastMs: VIDEO_POLL_MS,
      idleMs: VIDEO_IDLE_MS,
    });
  }, [working, refresh]);

  if (videos === undefined || videos === null) {
    return (
      <RelatedList title="Vídeos de esta partida">
        {videos === undefined ? (
          <p className="px-4 py-3 text-body-sm text-fg-3">Cargando vídeos</p>
        ) : (
          <p className="px-4 py-3 text-body-sm text-fg-2">No se pudieron leer los vídeos. Comprueba que ClipHub Studio sigue abierto.</p>
        )}
      </RelatedList>
    );
  }
  if (videos.length === 0) return <MakeVideos matchId={matchId} demo={demo} onOpenRounds={onOpenRounds} />;
  return <VideoList matchId={matchId} videos={videos} onChange={() => void refresh().catch(() => undefined)} />;
}

type MakePath = { id: string; icon: LucideIcon; title: string; format: string; description: string; action: ReactNode };

/** Each way to make a video of this partida. A path that needs the library demo or the Rondas tab shows only with it. */
function makePaths({ matchId, demo, onOpenRounds }: Omit<MatchVideosProps, 'videos' | 'onVideosChange'>): MakePath[] {
  const crear = ({ format, label, primary }: { format: ProduceFormat; label: string; primary: boolean }): ReactNode =>
    demo ? (
      <OpenInCrearButton request={{ demo, format }} variant={primary ? 'default' : 'outline'} size="sm">
        {label}
      </OpenInCrearButton>
    ) : (
      <Button asChild variant={primary ? 'default' : 'outline'} size="sm">
        <Link href={produceHref(matchId, format)}>{label}</Link>
      </Button>
    );
  const paths: MakePath[] = [
    {
      id: 'short',
      icon: Smartphone,
      title: 'Short vertical',
      format: '9:16, para TikTok, Reels y Shorts',
      description: 'Las mejores jugadas de un jugador, montadas en vertical.',
      action: crear({ format: PRODUCE_FORMAT.short, label: 'Crear Short', primary: true }),
    },
    {
      id: 'full',
      icon: MonitorPlay,
      title: 'Demo completa',
      format: '16:9, para YouTube',
      description: 'La partida entera desde los ojos de un jugador, con el HUD que elijas.',
      action: crear({ format: PRODUCE_FORMAT.full, label: 'Preparar demo completa', primary: false }),
    },
  ];
  if (demo) {
    paths.push({
      id: 'team',
      icon: Users,
      title: 'Lo mejor de un equipo',
      format: '16:9, varias vistas en un vídeo',
      description: 'Junta las mejores jugadas de 2 a 5 compañeros de equipo, cada una desde su vista.',
      action: (
        <OpenInCrearButton request={{ demo, format: PRODUCE_FORMAT.full, pickPlayers: true }} variant="outline" size="sm">
          Elegir jugadores
        </OpenInCrearButton>
      ),
    });
  }
  if (onOpenRounds) {
    paths.push({
      id: 'rounds',
      icon: Layers,
      title: 'Vídeo de rondas',
      format: 'Las rondas que tú elijas',
      description: 'Marca rondas en la pestaña Rondas, por ejemplo las pistolas o un retake, y haz un vídeo con todas.',
      action: (
        <Button type="button" variant="outline" size="sm" onClick={onOpenRounds}>
          Ir a Rondas
        </Button>
      ),
    });
  }
  return paths;
}

function MakeVideos(props: Omit<MatchVideosProps, 'videos' | 'onVideosChange'>): ReactNode {
  const headingId = useId();
  // A shared match came without its .dem, so no video can be made until the demo is imported.
  const shared = props.demo?.source === 'shared';
  return (
    <section aria-labelledby={headingId} className="studio-panel flex flex-col gap-4 p-5">
      <div className="flex flex-col gap-1">
        <h2 id={headingId} className="text-title font-semibold text-fg-1">
          Todavía no has hecho vídeos de esta partida
        </h2>
        <p className="text-body-sm text-fg-2">
          {shared ? SHARED_MATCH_REASON : 'Elige qué quieres sacar de ella. Los vídeos que hagas aparecerán aquí y en Mis vídeos.'}
        </p>
      </div>
      {shared ? null : (
        <ul className="grid grid-cols-[repeat(auto-fit,minmax(15rem,1fr))] gap-3">
          {makePaths(props).map(({ id, icon: Icon, title, format, description, action }) => (
            <li key={id} className="flex flex-col gap-3 rounded-lg border border-border-subtle bg-surface-2 p-4">
              <div className="flex items-center gap-2.5">
                <span aria-hidden className="grid size-9 shrink-0 place-items-center rounded-md bg-surface-3 text-fg-1">
                  <Icon className="size-4" />
                </span>
                <div className="flex min-w-0 flex-col">
                  <h3 className="text-body-sm font-semibold text-fg-1">{title}</h3>
                  <span className="text-meta text-fg-3">{format}</span>
                </div>
              </div>
              <p className="text-body-sm text-fg-2">{description}</p>
              <div className="mt-auto">{action}</div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** Newest first, with kind chips once the partida has more than one kind of video. */
function VideoList({ matchId, videos, onChange }: { matchId: string; videos: readonly Video[]; onChange: () => void }): ReactNode {
  const [kind, setKind] = useState<VideoKind | null>(null);
  const kinds = VIDEO_KINDS.filter((entry) => videos.some((video) => videoKind(video) === entry));
  // A kind whose last video was deleted falls back to every video.
  const active = kind !== null && kinds.includes(kind) ? kind : null;
  const listed = videos.filter((video) => active === null || videoKind(video) === active).sort((a, b) => b.createdAt - a.createdAt);
  const chips =
    kinds.length > 1 ? (
      <ToggleGroup
        type="single"
        variant="filter"
        size="sm"
        spacing={2}
        aria-label="Tipo de vídeo"
        className="flex-wrap"
        value={active ?? ALL_KINDS}
        onValueChange={(next: string) => setKind(VIDEO_KINDS.find((entry) => entry === next) ?? null)}
      >
        <ToggleGroupItem value={ALL_KINDS}>
          Todos <span className="font-normal text-fg-3">{videos.length}</span>
        </ToggleGroupItem>
        {kinds.map((entry) => (
          <ToggleGroupItem key={entry} value={entry}>
            {KIND_CHIP[entry]} <span className="font-normal text-fg-3">{videos.filter((video) => videoKind(video) === entry).length}</span>
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    ) : undefined;
  return (
    <RelatedList title="Vídeos de esta partida" actions={chips}>
      <ul className="divide-y divide-border-subtle">
        {listed.map((video) => (
          <VideoRow key={video.id} video={video} matchId={matchId} onChange={onChange} />
        ))}
      </ul>
    </RelatedList>
  );
}

/** One video: its cover, what it is, its progress or failure, and the Mis vídeos actions. The title opens its publish page. */
function VideoRow({ video, matchId, onChange }: { video: Video; matchId: string; onChange: () => void }): ReactNode {
  const output = toOutput(video);
  const jobId = video.jobId ?? matchId;
  const isShort = output.type === OUTPUT_TYPE.short;
  const playback = output.state === OUTPUT_STATE.ready ? demoPlaybackItem(video) : null;
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const play = (button: HTMLElement): void => {
    setReturnFocus(button);
    setPlayerOpen(true);
  };
  const state = VIDEO_STATE[output.state];
  const capturing = output.state === OUTPUT_STATE.rec || output.state === OUTPUT_STATE.render;
  const stage = output.state === OUTPUT_STATE.render ? video.captureProgress?.stage : undefined;
  const failure =
    output.state === OUTPUT_STATE.failed
      ? parseFailureReason(video.failureReason, {
          fullDemo: output.type === OUTPUT_TYPE.full && !isHighlightsVideo(video),
          fullDemoPlannerVersion: video.editConfig?.fullDemo?.document.planner_version,
        }).message
      : null;

  return (
    <li className="relative grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4 gap-y-3 px-4 py-3 hover:bg-surface-3 @[44rem]/content:grid-cols-[auto_minmax(0,1fr)_auto]">
      <div aria-hidden className="flex h-18 w-32 items-center justify-center overflow-hidden rounded-md border border-border-subtle bg-surface-0">
        <div className={cn('relative h-full overflow-hidden', isShort ? 'aspect-[9/16]' : 'aspect-video')}>
          <ReelCover seed={video.id} plain />
          <div className="absolute inset-0">
            <CoverImage src={video.thumbnailUrl} />
          </div>
        </div>
      </div>
      <div className="flex min-w-0 flex-col gap-1.5">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          {/* The link stretches over the row; the actions sit above it. */}
          <Link
            href={publishHref(jobId, video.id)}
            className={cn('min-w-0 truncate rounded-sm text-body-sm font-semibold text-fg-1 after:absolute after:inset-0 hover:underline', FOCUS_RING)}
          >
            {video.title}
          </Link>
          {output.state === OUTPUT_STATE.ready ? null : (
            <StatusTag tone={state.tone} dot>
              {state.label}
            </StatusTag>
          )}
        </div>
        <span className="truncate text-meta text-fg-3">
          {`${outputLabel(video)} ${isShort ? '9:16' : '16:9'} · ${stage ?? timeAgo(video.createdAt)}`}
        </span>
        {capturing ? (
          <div className="flex max-w-md items-center gap-2.5">
            <Progress
              size="sm"
              tone={output.state === OUTPUT_STATE.rec ? 'stream' : 'primary'}
              value={output.percent ?? 0}
              indeterminate={output.percent === null}
              aria-label={`Progreso de ${video.title}`}
              className="flex-1"
            />
            {output.percent === null ? null : <span className="text-meta whitespace-nowrap text-fg-2">{output.percent} %</span>}
          </div>
        ) : null}
        {failure === null ? null : <p className="text-body-sm text-fg-2">{failure}</p>}
      </div>
      <OutputActions
        output={output}
        matchId={jobId}
        onChange={onChange}
        onPlay={playback === null ? undefined : play}
        className="relative z-10 col-span-2 @[44rem]/content:col-span-1 @[44rem]/content:justify-end"
      />
      {playback ? (
        <MediaPlayer items={[playback]} activeId={playback.id} open={playerOpen} onOpenChange={setPlayerOpen} returnFocus={returnFocus} />
      ) : null}
    </li>
  );
}
