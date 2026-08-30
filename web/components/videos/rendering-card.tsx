'use client';

import type { ReactNode } from 'react';
import type { Video } from '@/lib/api/types';
import { captureProgressDetail, captureProgressPercent } from '@/lib/capture-progress';
import { jobProgressCount, jobProgressPercent } from '@/lib/job-progress';
import { isLandscapeRecap } from '@/lib/reel-brief';
import { RecDot } from '@/components/brand/rec-dot';
import { StatusTag } from '@/components/studio/status-tag';
import { ReelCard, reelFormatLabel, type ReelCardTone } from '@/components/videos/reel-card';

/** In-flight reel card. Capture and compose both show a real percent + count. */
export function RenderingCard({ video }: { video: Video }) {
  const isCapturing = video.status === 'recording';
  const isComposing = video.status === 'composing';

  const live = video.captureProgress;
  const capture =
    isCapturing && live && live.total > 0
      ? {
          detail: captureProgressDetail(live),
          pct: captureProgressPercent(live),
        }
      : undefined;
  const compose =
    isComposing && live
      ? {
          detail: jobProgressCount(live),
          pct: jobProgressPercent(live),
        }
      : undefined;
  const liveStat = capture ?? compose;

  const formatBadge = reelFormatLabel(video.editConfig);
  const fullDemo = video.editConfig != null && isLandscapeRecap(video.editConfig);

  let tone: ReelCardTone = 'neutral';
  let coverTint = 'bg-surface-0/55';
  let stageIndicator: ReactNode = <StatusTag>En cola</StatusTag>;
  // Queued jobs wait on the single cs2.exe capture lane.
  let detail = 'Esperando turno de captura';

  if (isCapturing) {
    tone = 'stream';
    coverTint = 'bg-gradient-to-br from-stream/25 via-surface-0/25 to-surface-0/70';
    // RecDot is the REC identity; it settles after three beats.
    stageIndicator = (
      <span className="inline-flex min-h-8 items-center border border-stream/45 bg-surface-0/85 px-2.5">
        <RecDot label="Capturando" />
      </span>
    );
    detail = capture ? capture.detail : 'Preparando captura local';
  } else if (isComposing) {
    tone = 'primary';
    coverTint = 'bg-gradient-to-br from-primary/18 via-surface-0/25 to-surface-0/70';
    stageIndicator = (
      <StatusTag tone="primary" dot>
        Editando
      </StatusTag>
    );
    detail = compose ? compose.detail : 'Montando cortes y ritmo';
  }

  return (
    <ReelCard
      video={video}
      tone={tone}
      percent={liveStat?.pct}
      plainCover
      coverClassName="opacity-55"
      coverTintClassName={coverTint}
      badge={
        formatBadge || fullDemo ? (
          <span className="flex flex-wrap items-center gap-1.5">
            {formatBadge ? <StatusTag>{formatBadge}</StatusTag> : null}
            {fullDemo ? <StatusTag tone="primary">PARTIDA COMPLETA</StatusTag> : null}
          </span>
        ) : undefined
      }
      frameFooter={stageIndicator}
    >
      {/* Percent is aria-hidden: the stage track progressbar already announces it. */}
      <div
        role="status"
        aria-live="polite"
        aria-atomic="true"
        aria-busy="true"
        className="flex items-end justify-between gap-3"
      >
        <span className="min-w-0 truncate font-mono text-meta uppercase text-fg-2">{detail}</span>
        {/* Capture is the only stage with a real percent, so the stat stays REC. */}
        {liveStat ? (
          <span aria-hidden className={`shrink-0 font-mono text-stat tabular-nums ${isCapturing ? 'text-stream-text' : 'text-primary'}`}>
            {liveStat.pct}%
          </span>
        ) : null}
      </div>
    </ReelCard>
  );
}
