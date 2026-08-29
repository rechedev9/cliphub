'use client';

import type { ReactNode } from 'react';
import type { Video } from '@/lib/api/types';
import { captureProgressDetail, captureProgressPercent } from '@/lib/capture-progress';
import { isLandscapeRecap } from '@/lib/reel-brief';
import { RecDot } from '@/components/brand/rec-dot';
import { StatusTag } from '@/components/studio/status-tag';
import { ReelCard, reelFormatLabel, type ReelCardTone } from '@/components/videos/reel-card';

/** In-flight reel card. Capture percent fills the strip; editing stays indeterminate. */
export function RenderingCard({ video }: { video: Video }) {
  const isCapturing = video.status === 'recording';
  const isComposing = video.status === 'composing';

  // One derived value so the strip, the stat, and the copy share a guard.
  const capture =
    isCapturing && video.captureProgress && video.captureProgress.total > 0
      ? {
          detail: captureProgressDetail(video.captureProgress),
          pct: captureProgressPercent(video.captureProgress),
        }
      : undefined;

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
    detail = 'Montando cortes y ritmo';
  }

  return (
    <ReelCard
      video={video}
      tone={tone}
      percent={capture?.pct}
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
        {capture ? (
          <span aria-hidden className="shrink-0 font-mono text-stat tabular-nums text-stream-text">
            {capture.pct}%
          </span>
        ) : null}
      </div>
    </ReelCard>
  );
}
