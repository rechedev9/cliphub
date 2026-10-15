'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Download, Play, RotateCcw } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import { OUTPUT_STATE, OUTPUT_TYPE, type MatchOutput } from '@/lib/clips/hub';
import { publishHref, produceHref, PRODUCE_FORMAT } from '@/lib/clips/routes';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { DeleteVideoButton } from '@/components/videos/delete-video-button';

const DOWNLOAD_TOAST = 'Descargando MP4';

type OutputItemProps = {
  output: MatchOutput;
  matchId: string;
  onChange: () => void;
};

type OutputActionsProps = OutputItemProps & {
  className?: string;
  onPlay?: (button: HTMLElement) => void;
  /** Icon-only Reproducir: the Clips lens card track cannot fit three labelled buttons on one row. */
  compact?: boolean;
};

/** Ready: MP4 + Publicar. Failed: Reintentar (when it can help) + delete. Queue/REC/render: delete only. */
export function OutputActions({ output, matchId, onChange, onPlay, className, compact = false }: OutputActionsProps): ReactNode {
  const { video } = output;
  const [retrying, setRetrying] = useState(false);

  if (output.state === OUTPUT_STATE.ready) {
    const url = video.downloadUrl;
    return (
      <span className={cn('flex flex-wrap items-center gap-1.5', className)}>
        {onPlay ? (
          <Button
            type="button"
            size={compact ? 'icon-xs' : 'xs'}
            variant="outline-primary"
            aria-label={compact ? 'Reproducir' : undefined}
            title={compact ? 'Reproducir' : undefined}
            onClick={(event) => onPlay(event.currentTarget)}
          >
            <Play aria-hidden />
            {compact ? null : 'Reproducir'}
          </Button>
        ) : null}
        <Button
          type="button"
          size="xs"
          variant="outline-primary"
          disabled={url === undefined}
          onClick={() => {
            if (url === undefined) return;
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
    const failure = parseFailureReason(video.failureReason, { fullDemo: output.type === OUTPUT_TYPE.full, fullDemoPlannerVersion: video.editConfig?.fullDemo?.document.planner_version });
    const canRetry = video.unrecoverable !== true && failure.retryCanHelp;
    return (
      <span className={cn('flex flex-wrap items-center gap-1.5', className)}>
        {video.unrecoverable !== true && (failure.kind === 'pov-acquisition' || failure.kind === 'pov-verification') ? (
          <Button asChild size="xs" variant="outline">
            <Link href={produceHref(matchId, PRODUCE_FORMAT.full)}>Volver a preparar</Link>
          </Button>
        ) : null}
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
