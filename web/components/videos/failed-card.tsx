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

// Failed reels keep the shared card system. Retry re-drives recoverable stages;
// unrecoverable cards instead direct the user to delete and prepare again.
export function FailedCard({ video, onChange }: { video: Video; onChange: () => void }) {
  const [retrying, setRetrying] = useState(false);
  const [retryError, setRetryError] = useState<string | null>(null);
  const unrecoverable = video.unrecoverable ?? false;
  const failure = parseFailureReason(video.failureReason, {
    fullDemo: video.editConfig?.matchRecap === true,
  });
  // The classifier hides Retry for deterministic startup/demo/stale-plan failures.
  const demoIncompatible = !unrecoverable && failure.kind === 'demo-incompatible';
  const canRetry = !unrecoverable && failure.retryCanHelp;
  const formatBadge = reelFormatLabel(video.editConfig);

  function footerHint(): string {
    if (unrecoverable) return 'Elimina la tarjeta y sube la demo otra vez para forjarla de nuevo.';
    if (demoIncompatible) {
      return 'Este fallo es determinista: reintentar no ayudará. Elimina la tarjeta o forja con una demo reciente.';
    }
    if (failure.kind === 'pov-verification') {
      return 'Elimina esta tarjeta y vuelve a preparar la demo para regenerar el plan de rondas.';
    }
    if (failure.kind === 'capture-flake') {
      return 'Reintenta: no es un pipeline muerto. Si el POV no vuelve, el vídeo no se publicará con otro jugador.';
    }
    if (!failure.retryCanHelp) return 'Reintentar el mismo trabajo no resolverá este fallo.';
    return 'Reintenta para retomar desde la etapa que falló.';
  }

  async function onRetry() {
    if (retrying) return;
    setRetrying(true);
    setRetryError(null);
    try {
      await api.retryVideo(video.id);
      onChange();
    } catch (err) {
      setRetryError(err instanceof Error ? err.message : 'No se pudo reintentar el vídeo.');
    } finally {
      // Re-arm even if the reel stays failed, so another attempt needs no reload.
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
      {retryError ? (
        <p role="alert" className="border-t border-destructive/25 pt-2.5 text-body-sm text-destructive">
          {retryError}
        </p>
      ) : null}
    </ReelCard>
  );
}
