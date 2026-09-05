'use client';

import { memo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, Play, RotateCcw } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { demoPlaybackItem } from '@/lib/api/playback';
import { parseFailureReason } from '@/lib/api/failure-reason';
import {
  isWorking,
  OUTPUT_STATE,
  OUTPUT_TYPE,
  sameHubProps,
  type MatchOutput,
  type OutputState,
} from '@/lib/clips/hub';
import { publishHref } from '@/lib/clips/routes';
import { timeAgo } from '@/lib/format';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { Button } from '@/components/ui/button';
import { MediaPlayer } from '@/components/studio/media-player';
import { DeleteVideoButton } from '@/components/videos/delete-video-button';
import { OutputTag } from '@/components/clips-hub/output-tag';

const BORDER_CLASS = {
  ready: 'border-success/45',
  rec: 'border-stream/45',
  render: 'border-border-accent',
  queue: 'border-border',
  failed: 'border-destructive/45',
} as const satisfies Record<OutputState, string>;

const TEXT_CLASS = {
  ready: 'text-success',
  rec: 'text-stream-text',
  render: 'text-primary',
  queue: 'text-fg-3',
  failed: 'text-destructive',
} as const satisfies Record<OutputState, string>;

const DOWNLOAD_TOAST = 'Descargando MP4';

export type OutputItemProps = {
  output: MatchOutput;
  matchId: string;
  onChange: () => void;
};

type OutputActionsProps = OutputItemProps & { className?: string; onPlay?: (button: HTMLElement) => void };

/** One Short or Full POV inside an open partida row. */
function OutputItemCard({ output, matchId, onChange }: OutputItemProps): ReactNode {
  const { video } = output;
  const isShort = output.type === OUTPUT_TYPE.short;
  const playback = output.state === OUTPUT_STATE.ready ? demoPlaybackItem(video) : null;
  const [playerOpen, setPlayerOpen] = useState(false);
  const [returnFocus, setReturnFocus] = useState<HTMLElement | null>(null);
  const play = (button: HTMLElement): void => {
    setReturnFocus(button);
    setPlayerOpen(true);
  };
  return (
    <>
      <div
        className={cn(
          'studio-enter flex flex-wrap items-center gap-3 rounded-lg border bg-surface-2 px-3 py-2.5 transition-colors duration-(--dur-base)',
          BORDER_CLASS[output.state],
        )}
      >
        <button
          type="button"
          disabled={playback === null}
          aria-label={playback === null ? video.title : `Reproducir ${video.title}`}
          onClick={(event) => play(event.currentTarget)}
          className={cn(
            'relative shrink-0 overflow-hidden border focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default',
            isShort ? 'h-[50px] w-7' : 'h-[47px] w-[84px]',
            BORDER_CLASS[output.state],
          )}
        >
          <ReelCover seed={video.id} plain />
          <span className="absolute inset-0">
            <CoverImage src={video.thumbnailUrl} />
          </span>
        </button>

        <span className="flex min-w-[10rem] flex-1 flex-col gap-1.5">
          <span className="flex items-center gap-2">
            <span className="min-w-0 truncate font-display text-label font-bold uppercase text-fg-1">{output.title}</span>
            <OutputTag output={output} className="shrink-0" />
          </span>

          {isWorking(output.state) ? (
            <span className={cn('studio-bar', TEXT_CLASS[output.state])}>
              <span
                className={output.percent === null ? 'studio-indeterminate' : undefined}
                style={output.percent === null ? undefined : { width: `${output.percent}%` }}
              />
            </span>
          ) : null}

          {output.state === OUTPUT_STATE.failed ? (
            <FailureLine output={output} />
          ) : (
            <span className="truncate font-mono text-meta uppercase tracking-wider text-fg-3">
              {`${video.map} · ${timeAgo(video.createdAt)}`}
            </span>
          )}
        </span>

        <OutputActions
          output={output}
          matchId={matchId}
          onChange={onChange}
          onPlay={playback === null ? undefined : play}
          className="row-actions ml-auto w-full @[44rem]/content:w-auto"
        />
      </div>
      {playback ? (
        <MediaPlayer
          items={[playback]}
          activeId={playback.id}
          open={playerOpen}
          onOpenChange={setPlayerOpen}
          returnFocus={returnFocus}
        />
      ) : null}
    </>
  );
}

/** The hub rebuilds its model on every poll, so props compare by value, not identity. */
export const OutputItem = memo(OutputItemCard, sameHubProps);

function FailureLine({ output }: { output: MatchOutput }): ReactNode {
  const failure = parseFailureReason(output.video.failureReason, { fullDemo: output.type === OUTPUT_TYPE.full });
  return (
    <span className="text-body-sm text-destructive">{failure.message}</span>
  );
}

/** Ready: MP4 + Publicar. Failed: Reintentar (when it can help) + delete. Queue/REC/render: delete only. */
export function OutputActions({ output, matchId, onChange, onPlay, className }: OutputActionsProps): ReactNode {
  const { video } = output;
  const [retrying, setRetrying] = useState(false);

  if (output.state === OUTPUT_STATE.ready) {
    const url = video.downloadUrl;
    // Review pending: Publicar hosts the sign-off, so the raw MP4 stays locked.
    const blocked = url === undefined || output.reviewRequired;
    return (
      <span className={cn('flex flex-wrap items-center gap-1.5', className)}>
        {onPlay ? (
          <Button type="button" size="xs" variant="outline-primary" onClick={(event) => onPlay(event.currentTarget)}>
            <Play aria-hidden />
            Reproducir
          </Button>
        ) : null}
        <Button
          type="button"
          size="xs"
          variant="outline-primary"
          disabled={blocked}
          onClick={() => {
            if (url === undefined || output.reviewRequired) return;
            downloadPublishMP4(url, video.title);
            toast(DOWNLOAD_TOAST, { description: video.title });
          }}
        >
          <Download aria-hidden />
          MP4
        </Button>
        <Button asChild size="xs" variant="outline">
          <Link href={publishHref(matchId, video.id)}>Publicar</Link>
        </Button>
      </span>
    );
  }

  if (output.state === OUTPUT_STATE.failed) {
    const failure = parseFailureReason(video.failureReason, { fullDemo: output.type === OUTPUT_TYPE.full });
    const canRetry = video.unrecoverable !== true && failure.retryCanHelp;
    return (
      <span className={cn('flex flex-wrap items-center gap-1.5', className)}>
        {canRetry ? (
          <Button
            type="button"
            size="xs"
            variant="outline"
            loading={retrying}
            onClick={() => {
              setRetrying(true);
              void api
                .retryVideo(video.id)
                .then(onChange)
                .catch(() => toast('No se pudo reintentar', { description: video.title }))
                .finally(() => setRetrying(false));
            }}
          >
            <RotateCcw aria-hidden />
            Reintentar
          </Button>
        ) : null}
        <DeleteVideoButton video={video} onDeleted={onChange} />
      </span>
    );
  }

  // Queue/REC/render: still no result, but the intent must stay removable.
  return (
    <span className={cn('flex flex-wrap items-center gap-1.5', className)}>
      <DeleteVideoButton video={video} onDeleted={onChange} />
    </span>
  );
}
