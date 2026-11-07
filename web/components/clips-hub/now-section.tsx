'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { ArrowRight, Film, TriangleAlert } from 'lucide-react';
import { prettifyMap } from '@/lib/api/map';
import { isHighlightsVideo, OUTPUT_STATE, OUTPUT_TYPE, outputLabel, outputTagLabel, type HubModel } from '@/lib/clips/hub';
import type { StreamActivity } from '@/lib/clips/results';
import { ORPHAN_MATCH_SEGMENT, PRODUCE_FORMAT, produceHref } from '@/lib/clips/routes';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { FORMAT_TONE } from '@/components/studio/format-tone';
import { Button } from '@/components/ui/button';
import { FailureLine, OutputActions } from '@/components/clips-hub/output-item';
import { OutputTag } from '@/components/clips-hub/output-tag';

type HubClip = HubModel['clips'][number];

/** Past this many rounds a tick per round gets thinner than the gap; the plain bar reads better. */
const MAX_TICKS = 40;

const HEADING_ID = 'mis-videos-ahora';

const LINK_CLASS =
  'inline-flex h-8 items-center gap-1 rounded-full px-2.5 text-meta text-fg-3 transition-colors duration-(--dur-fast) hover:text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring';

export type NowSectionProps = {
  /** Clips on REC or render, newest first; the first one is the hero. */
  running: readonly HubClip[];
  queued: readonly HubClip[];
  failed: readonly HubClip[];
  /** Stream projects downloading or exporting; each row opens its project. */
  streams: readonly StreamActivity[];
  onChange: () => void;
};

/** "Grabando en este PC": what this PC is doing now, then what waits and what failed. */
export function NowSection({ running, queued, failed, streams, onChange }: NowSectionProps): ReactNode {
  const [hero, ...otherRunning] = running;
  const sideCount = otherRunning.length + streams.length + queued.length + failed.length;
  if (hero === undefined && sideCount === 0) return null;
  return (
    <section
      aria-labelledby={HEADING_ID}
      className={cn('grid gap-4', hero !== undefined && sideCount > 0 && '@[56rem]/content:grid-cols-[minmax(0,1.6fr)_minmax(17.5rem,1fr)]')}
    >
      {hero === undefined ? (
        <h2 id={HEADING_ID} className="sr-only">Pendientes en este PC</h2>
      ) : (
        <NowHero clip={hero} onChange={onChange} />
      )}
      {sideCount > 0 ? (
        <ul className="flex flex-col gap-2.5">
          {otherRunning.map((clip) => (
            <SideRow key={clip.id} clip={clip} onChange={onChange} />
          ))}
          {streams.map((activity) => (
            <StreamRow key={activity.jobId} activity={activity} />
          ))}
          {[...queued, ...failed].map((clip) => (
            <SideRow key={clip.id} clip={clip} onChange={onChange} />
          ))}
        </ul>
      ) : null}
    </section>
  );
}

/** "Mirage · donk · 9:16": the facts that tell two videos of the same kind apart. */
function clipDetail(clip: HubClip): string {
  const { video } = clip;
  const format = clip.type === OUTPUT_TYPE.short ? '9:16' : '16:9';
  return [prettifyMap(video.map), clip.match?.player ?? video.targetName, format].filter(Boolean).join(' · ');
}

function CreateAnotherLink({ clip }: { clip: HubClip }): ReactNode {
  const matchId = clip.match?.id;
  if (matchId === undefined) return null;
  const isShort = clip.type === OUTPUT_TYPE.short;
  return (
    <Link
      href={produceHref(matchId, isShort ? PRODUCE_FORMAT.short : PRODUCE_FORMAT.full)}
      aria-label={isShort ? 'Crear otro Short de esta demo' : 'Crear otra demo completa de esta partida'}
      className={LINK_CLASS}
    >
      Crear otro
      <ArrowRight aria-hidden className="size-3" />
    </Link>
  );
}

function NowHero({ clip, onChange }: { clip: HubClip; onChange: () => void }): ReactNode {
  const { video } = clip;
  const recording = clip.state === OUTPUT_STATE.rec;
  const stage = video.captureProgress?.stage;
  return (
    <div className={cn('overflow-hidden rounded-[20px] border bg-surface-2', FORMAT_TONE.stream.border)}>
      <div className={cn('flex h-full flex-col gap-4 p-5', FORMAT_TONE.stream.tint)}>
        <div className="flex flex-wrap items-center gap-3">
          <span aria-hidden className="size-3 shrink-0 rounded-full bg-format-stream ring-4 ring-format-stream/30" />
          <h2 id={HEADING_ID} className="text-label font-bold text-fg-2">
            {recording ? 'Grabando en este PC' : 'Editando en este PC'}
          </h2>
          <span className="ml-auto flex items-center gap-1.5">
            <CreateAnotherLink clip={clip} />
            <OutputActions output={clip} matchId={clip.match?.id ?? ORPHAN_MATCH_SEGMENT} onChange={onChange} />
          </span>
        </div>
        <div className="flex items-center gap-3.5">
          <RowThumbnail clip={clip} />
          <div className="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-3.5 gap-y-1">
            <span className="min-w-0 flex-[1_1_18rem] font-display text-section font-bold wrap-anywhere text-fg-1">{clip.title}</span>
            <span className="text-body text-fg-2">
              {outputLabel(video)} · {clipDetail(clip)}
              {stage === undefined ? null : ` · ${stage}`}
            </span>
          </div>
        </div>
        <HeroProgress clip={clip} />
      </div>
    </div>
  );
}

function HeroProgress({ clip }: { clip: HubClip }): ReactNode {
  const { rounds } = clip;
  if (clip.state === OUTPUT_STATE.rec && rounds !== null && rounds.total <= MAX_TICKS) {
    const unit = clip.type === OUTPUT_TYPE.full && !isHighlightsVideo(clip.video) ? 'Ronda' : 'Jugada';
    const label = `${unit} ${rounds.done} de ${rounds.total}`;
    return (
      <div className="flex flex-col gap-1.5">
        <div
          role="img"
          aria-label={`${label} grabada`}
          className="grid gap-[3px]"
          style={{ gridTemplateColumns: `repeat(${rounds.total}, minmax(0, 1fr))` }}
        >
          {Array.from({ length: rounds.total }, (_, index) => (
            <span
              key={index}
              className={cn(
                'h-3.5 rounded-[3px]',
                tickClass(index, rounds.done),
              )}
            />
          ))}
        </div>
        <span className="text-meta text-fg-2">{label}</span>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-1.5">
      <span className={cn('studio-bar', clip.state === OUTPUT_STATE.rec ? 'text-stream-text' : 'text-primary')}>
        <span
          className={clip.percent === null ? 'studio-indeterminate' : undefined}
          style={clip.percent === null ? undefined : { width: `${clip.percent}%` }}
        />
      </span>
      <span className="text-meta text-fg-2">{outputTagLabel(clip)}</span>
    </div>
  );
}

/** Recorded rounds solid, the one on capture half tinted, the rest empty. */
function tickClass(index: number, done: number): string {
  if (index < done) return FORMAT_TONE.stream.bg;
  return index === done ? 'bg-format-stream/45' : 'bg-surface-4';
}

/** The video's poster in its real shape inside one fixed slot, so row text lines up. */
function RowThumbnail({ clip }: { clip: HubClip }): ReactNode {
  const failed = clip.state === OUTPUT_STATE.failed;
  return (
    <span aria-hidden className="grid h-12 w-[4.25rem] shrink-0 place-items-center">
      <span
        className={cn(
          'relative block overflow-hidden rounded-[8px] bg-surface-4',
          clip.type === OUTPUT_TYPE.short ? 'h-12 w-[1.6875rem]' : 'h-[2.375rem] w-[4.25rem]',
        )}
      >
        <ReelCover seed={clip.video.id} plain />
        <span className="absolute inset-0">
          <CoverImage src={clip.video.thumbnailUrl} />
        </span>
        {failed ? (
          <span className="absolute inset-0 grid place-items-center bg-destructive-solid/75 text-white">
            <TriangleAlert className="size-4" />
          </span>
        ) : null}
      </span>
    </span>
  );
}

function SideRow({ clip, onChange }: { clip: HubClip; onChange: () => void }): ReactNode {
  const failed = clip.state === OUTPUT_STATE.failed;
  return (
    <li
      className={cn(
        'flex flex-wrap items-center gap-3 rounded-[14px] px-3.5 py-3',
        failed ? 'border border-destructive/45 bg-destructive/10' : 'bg-surface-2',
      )}
    >
      <RowThumbnail clip={clip} />
      <span className="flex min-w-[10rem] flex-1 flex-col gap-0.5">
        <span className="truncate text-body-sm font-bold text-fg-1">{clip.title}</span>
        <SideRowMeta clip={clip} />
        <span className="truncate text-meta text-fg-3">{clipDetail(clip)}</span>
      </span>
      <span className="ml-auto flex items-center gap-1.5">
        <CreateAnotherLink clip={clip} />
        <OutputActions output={clip} matchId={clip.match?.id ?? ORPHAN_MATCH_SEGMENT} onChange={onChange} />
      </span>
    </li>
  );
}

function SideRowMeta({ clip }: { clip: HubClip }): ReactNode {
  if (clip.state === OUTPUT_STATE.failed) return <FailureLine output={clip} className="text-meta text-fg-2" />;
  if (clip.state === OUTPUT_STATE.queue) {
    return <span className="text-meta text-fg-3">En cola · {outputLabel(clip.video)}</span>;
  }
  return (
    <span className="flex items-center gap-2">
      <OutputTag output={clip} className="shrink-0" />
      <span className="truncate text-meta text-fg-3">{outputLabel(clip.video)}</span>
    </span>
  );
}

function streamActivityLabel(activity: StreamActivity): string {
  const parts = [activity.stage === 'download' ? 'Descargando' : 'Exportando'];
  if (activity.clipLabel !== null) parts.push(activity.clipLabel);
  if (activity.percent !== null) parts.push(`${activity.percent}%`);
  return parts.join(' · ');
}

/** A stream project at work: its own page holds the editor, so the row only reports and links. */
function StreamRow({ activity }: { activity: StreamActivity }): ReactNode {
  const label = streamActivityLabel(activity);
  return (
    <li className="flex flex-wrap items-center gap-3 rounded-[14px] bg-surface-2 px-3.5 py-3">
      <span aria-hidden className="grid h-12 w-[4.25rem] shrink-0 place-items-center">
        <span className={cn('grid h-[2.375rem] w-[4.25rem] place-items-center rounded-[8px] text-format-stream', FORMAT_TONE.stream.tint)}>
          <Film className="size-4" />
        </span>
      </span>
      <span className="flex min-w-[10rem] flex-1 flex-col gap-1">
        <span className="truncate text-body-sm font-bold text-fg-1">{activity.title}</span>
        <span className="truncate text-meta text-fg-3">Proyecto de stream · {label}</span>
        <span aria-hidden className="studio-bar text-stream-text">
          <span
            className={activity.percent === null ? 'studio-indeterminate' : undefined}
            style={activity.percent === null ? undefined : { width: `${activity.percent}%` }}
          />
        </span>
      </span>
      <Button asChild size="xs" variant="outline" className="ml-auto">
        <Link href={`/streams/${activity.jobId}`} aria-label={`Ver progreso de ${activity.title}`}>
          Ver progreso
          <ArrowRight aria-hidden />
        </Link>
      </Button>
    </li>
  );
}
