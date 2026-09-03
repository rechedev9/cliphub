'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, RotateCcw } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import { isWorking, OUTPUT_STATE, OUTPUT_TYPE, type MatchOutput, type OutputState } from '@/lib/clips/hub';
import { publishHref } from '@/lib/clips/routes';
import { timeAgo } from '@/lib/format';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import { cn } from '@/lib/utils';
import { ReelCover } from '@/components/brand/reel-cover';
import { CoverImage } from '@/components/studio/cover-image';
import { Button } from '@/components/ui/button';
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

/** One Short or Full POV inside an open partida row. */
export function OutputItem({ output, matchId, onChange }: OutputItemProps): ReactNode {
  const { video } = output;
  const isShort = output.type === OUTPUT_TYPE.short;
  return (
    <div
      className={cn(
        'studio-enter flex items-center gap-3 rounded-lg border bg-surface-2 px-3 py-2.5 transition-colors duration-(--dur-base)',
        BORDER_CLASS[output.state],
      )}
    >
      <span
        aria-hidden
        className={cn(
          'relative shrink-0 overflow-hidden border',
          isShort ? 'h-[50px] w-7' : 'h-[47px] w-[84px]',
          BORDER_CLASS[output.state],
        )}
      >
        <ReelCover seed={video.id} plain />
        <span className="absolute inset-0">
          <CoverImage src={video.thumbnailUrl} />
        </span>
      </span>

      <span className="flex min-w-0 flex-1 flex-col gap-1.5">
        <span className="flex items-center justify-between gap-2">
          <span className="truncate font-display text-label font-bold uppercase text-fg-1">{output.title}</span>
          <OutputTag output={output} />
        </span>

        {isWorking(output.state) ? (
          <span className={cn('studio-bar', TEXT_CLASS[output.state])}>
            <span
              className={output.percent === null ? 'studio-indeterminate' : undefined}
              style={output.percent === null ? undefined : { width: `${output.percent}%` }}
            />
          </span>
        ) : null}

        <span className="flex justify-between gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
          <span className="truncate">
            {output.state === OUTPUT_STATE.failed ? <FailureLine output={output} /> : `${video.map} · ${timeAgo(video.createdAt)}`}
          </span>
        </span>

        <OutputActions output={output} matchId={matchId} onChange={onChange} />
      </span>
    </div>
  );
}

function FailureLine({ output }: { output: MatchOutput }): ReactNode {
  const failure = parseFailureReason(output.video.failureReason, { fullDemo: output.type === OUTPUT_TYPE.full });
  return (
    <span className="normal-case tracking-normal text-destructive" title={failure.message}>
      {failure.message}
    </span>
  );
}

/** Ready: MP4 + Publicar. Failed: Reintentar (when it can help) + delete. Working: nothing. */
export function OutputActions({ output, matchId, onChange, className }: OutputItemProps & { className?: string }): ReactNode {
  const { video } = output;
  const [retrying, setRetrying] = useState(false);

  if (output.state === OUTPUT_STATE.ready) {
    const url = video.downloadUrl;
    // Review pending: Publicar hosts the sign-off, so the raw MP4 stays locked.
    const blocked = url === undefined || output.reviewRequired;
    return (
      <span className={cn('flex flex-wrap items-center gap-1.5', className)}>
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

  return null;
}
