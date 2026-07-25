'use client';

import { useState } from 'react';
import { AlertTriangle, RotateCcw } from 'lucide-react';
import type { Video } from '@/lib/api/types';
import { api } from '@/lib/api';
import { parseFailureReason } from '@/lib/api/failure-reason';
import { Button } from '@/components/ui/button';
import { StatusTag } from '@/components/studio/status-tag';
import { DeleteVideoButton } from '@/components/videos/delete-video-button';
import { ReelCard, reelFormatLabel } from '@/components/videos/reel-card';

/**
 * A reel whose pipeline failed on the rig. It is the same tile as every other
 * reel — same frame, same format-driven shape, same instrument strip — carrying
 * a destructive edge and an inline retry, instead of the full-width horizontal
 * row it used to be, which made the Library two unrelated component systems
 * stacked on top of each other.
 *
 * Retry re-drives the failed stage (re-record or re-render); on success the reel
 * rejoins the pipeline via the reconcile loop. When the reel is unrecoverable
 * (its orchestrator job is gone) Retry could never succeed, so the card hides it
 * and points the user at delete + re-forge.
 */
export function FailedCard({ video, onChange }: { video: Video; onChange: () => void }) {
  const [retrying, setRetrying] = useState(false);
  const unrecoverable = video.unrecoverable ?? false;
  const failure = parseFailureReason(video.failureReason);
  // A demo-incompatible failure is deterministic in the .dem itself: retry can
  // never help, so we hide Retry and show the Spanish explanation. Unrecoverable
  // reels keep their own branch.
  const demoIncompatible = !unrecoverable && failure.kind === 'demo-incompatible';
  const canRetry = !unrecoverable && !demoIncompatible;
  const formatBadge = reelFormatLabel(video.editConfig);

  function footerHint(): string {
    if (unrecoverable) return 'Elimina la tarjeta y sube la demo otra vez para forjarla de nuevo.';
    if (demoIncompatible) {
      return 'Este fallo es determinista: reintentar no ayudará. Elimina la tarjeta o forja con una demo reciente.';
    }
    return 'Reintenta para retomar desde la etapa que falló';
  }

  async function onRetry() {
    if (retrying) return;
    setRetrying(true);
    try {
      await api.retryVideo(video.id);
      onChange();
    } finally {
      // Always re-arm the button: a retry can resolve while the reel stays
      // failed (e.g. capture still unconfigured), and a card stuck at
      // "Reintentando…" would need a reload to try again.
      setRetrying(false);
    }
  }

  return (
    <ReelCard
      video={video}
      tone="danger"
      coverClassName="opacity-30"
      coverTintClassName="bg-gradient-to-br from-destructive/25 via-surface-0/40 to-surface-0/75"
      badge={formatBadge ? <StatusTag>{formatBadge}</StatusTag> : undefined}
      frameFooter={
        <StatusTag tone="danger" icon={AlertTriangle}>
          Fallo
        </StatusTag>
      }
      footer={
        <div className="flex items-center justify-end gap-2 p-4">
          {canRetry ? (
            <Button
              type="button"
              variant="outline"
              className="min-w-0 flex-1 border-destructive/45 text-destructive hover:border-destructive hover:bg-destructive/12 hover:text-destructive"
              onClick={onRetry}
              loading={retrying}
            >
              {retrying ? (
                'Reintentando…'
              ) : (
                <>
                  <RotateCcw className="size-4" aria-hidden /> Reintentar
                </>
              )}
            </Button>
          ) : null}
          <DeleteVideoButton video={video} onDeleted={onChange} />
        </div>
      }
    >
      <p className="line-clamp-3 text-body-sm text-destructive">
        {/* Unrecoverable reels carry an internal English reason; show the Spanish
            explanation instead of leaking it into the UI. A demo-incompatible
            reason is machine-readable, so we surface its parsed Spanish message
            rather than the raw prefix. */}
        {unrecoverable
          ? 'El orquestador ya no tiene esta captura (puede haberse reiniciado).'
          : failure.message}
      </p>
      <p className="border-t border-destructive/25 pt-2.5 font-mono text-meta uppercase text-fg-3">
        {footerHint()}
      </p>
    </ReelCard>
  );
}
