'use client';

import type { ReactNode } from 'react';
import type { Video } from '@/lib/api/types';
import { RecDot } from '@/components/brand/rec-dot';
import { StatusTag } from '@/components/studio/status-tag';
import { ReelCard, reelFormatLabel, type ReelCardTone } from '@/components/videos/reel-card';

/**
 * A render still in flight (queued / recording / composing). Same card as a
 * finished reel — same frame, same format-driven shape, same instrument strip —
 * with the stage carried by the edge tone, the indicator over the cover and the
 * strip's running segment.
 *
 * While capturing, the orchestrator reports real segment progress (done/total),
 * so the strip fills to the true percentage and the card prints it at stat size.
 * Until the first segment lands — and for the editing stage, which has no such
 * signal — the segment sweeps and no number is shown, because there is none.
 */
export function RenderingCard({ video }: { video: Video }) {
  const isCapturing = video.status === 'recording';
  const isComposing = video.status === 'composing';

  // Real capture progress, present only while capturing and once at least one
  // segment clip exists. A single derived value carries done, total, and the
  // percent together so the JSX guards on one thing.
  const capture =
    isCapturing && video.captureProgress && video.captureProgress.total > 0
      ? {
          done: video.captureProgress.done,
          total: video.captureProgress.total,
          pct: Math.min(
            100,
            Math.max(0, Math.round((video.captureProgress.done / video.captureProgress.total) * 100)),
          ),
        }
      : undefined;

  const formatBadge = reelFormatLabel(video.editConfig);

  let tone: ReelCardTone = 'neutral';
  let coverTint = 'bg-surface-0/55';
  let stageIndicator: ReactNode = <StatusTag>En cola</StatusTag>;
  let detail = 'Esperando captura';

  if (isCapturing) {
    tone = 'stream';
    coverTint = 'bg-gradient-to-br from-stream/25 via-surface-0/25 to-surface-0/70';
    // RecDot, not a StatusTag: the pulsing magenta LED is the REC identity, and
    // it settles to a static indicator after three beats.
    stageIndicator = (
      <span className="inline-flex min-h-8 items-center border border-stream/45 bg-surface-0/85 px-2.5">
        <RecDot label="Capturando" />
      </span>
    );
    detail = capture ? `Segmentos ${capture.done}/${capture.total}` : 'Preparando captura';
  } else if (isComposing) {
    tone = 'primary';
    coverTint = 'bg-gradient-to-br from-primary/18 via-surface-0/25 to-surface-0/70';
    stageIndicator = (
      <StatusTag tone="primary" dot>
        Editando
      </StatusTag>
    );
    detail = 'Cortes + ritmo';
  }

  return (
    <ReelCard
      video={video}
      tone={tone}
      percent={capture?.pct}
      coverClassName="opacity-55"
      coverTintClassName={coverTint}
      badge={formatBadge ? <StatusTag>{formatBadge}</StatusTag> : undefined}
      frameFooter={stageIndicator}
    >
      {/*
        role="status" so a stage change on a multi-minute local job is announced
        without stealing focus. The percentage is aria-hidden: the stage track's
        progressbar already exposes the same number, and announcing both would
        read it twice on every poll tick.
      */}
      <div role="status" className="flex items-end justify-between gap-3">
        <span className="min-w-0 truncate font-mono text-meta uppercase text-fg-2">{detail}</span>
        {/* `capture` only ever exists while recording, so the stat is always the
            REC colour; there is no other stage with a real percentage. */}
        {capture ? (
          <span aria-hidden className="shrink-0 font-mono text-stat tabular-nums text-stream-text">
            {capture.pct}%
          </span>
        ) : null}
      </div>
    </ReelCard>
  );
}
