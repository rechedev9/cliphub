'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, Captions, CircleCheck, Loader2, RefreshCw } from 'lucide-react';
import {
  CAPTION_GENERATION_STATUS,
  type CaptionGenerationState,
  type StreamCaptionWord,
  type StreamClipRange,
} from '@/lib/api/streams';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { StreamAnalysisProgress } from '@/components/streams/analysis-progress';
import { StreamCaptionReviewCard } from '@/components/streams/caption-review-card';

/**
 * Burned-in Spanish captions: the generation gate and the per-clip review.
 *
 * The panel never lets a machine transcript look finished — the review state is
 * a status tag next to the section head, and the render stays blocked until
 * every audible clip is approved or confirmed silent.
 */
export function StreamCaptionsPanel({
  enabled,
  clips,
  videoSrc,
  captionState,
  captionDrafts,
  captionError,
  captionLoading,
  captionRequestBusy,
  captionGenerating,
  captionReviewBlocked,
  canGenerateCaptions,
  sourceHasAudio,
  reviewingClipId,
  busy,
  onToggle,
  onGenerate,
  onCancelWait,
  onWordsChange,
  onApprove,
  onNoSpeech,
}: {
  enabled: boolean;
  clips: StreamClipRange[];
  videoSrc: string;
  captionState: CaptionGenerationState | null;
  captionDrafts: Record<string, StreamCaptionWord[]>;
  captionError: string | null;
  captionLoading: boolean;
  captionRequestBusy: boolean;
  captionGenerating: boolean;
  captionReviewBlocked: boolean;
  canGenerateCaptions: boolean;
  sourceHasAudio: boolean;
  reviewingClipId: string | null;
  busy: boolean;
  onToggle: (enabled: boolean) => void;
  onGenerate: () => void;
  onCancelWait: () => void;
  onWordsChange: (clipId: string, words: StreamCaptionWord[]) => void;
  onApprove: (clip: StreamClipRange) => void;
  onNoSpeech: (clip: StreamClipRange) => void;
}): ReactNode {
  let notice: ReactNode;
  if (!sourceHasAudio) {
    notice = (
      <p className="flex items-center gap-2 text-body-sm text-success">
        <CircleCheck aria-hidden className="size-4 shrink-0" />
        El archivo no tiene pista de audio; no necesita revisión de subtítulos.
      </p>
    );
  } else if (captionGenerating || captionRequestBusy) {
    notice = <StreamAnalysisProgress label="Analizando audio por clip" onCancel={onCancelWait} />;
  } else if (captionLoading) {
    notice = (
      <p role="status" className="flex items-center gap-2 text-body-sm text-fg-2">
        <Loader2 aria-hidden className="size-4 shrink-0 animate-spin" />
        Consultando candidatos guardados…
      </p>
    );
  } else if (!captionReviewBlocked) {
    notice = (
      <p role="status" className="flex items-center gap-2 text-body-sm text-success">
        <CircleCheck aria-hidden className="size-4 shrink-0" />
        Todos los clips con audio están revisados.
      </p>
    );
  } else if (captionState?.status === CAPTION_GENERATION_STATUS.failed) {
    notice = (
      <p role="alert" className="flex items-start gap-2 border border-destructive/45 bg-destructive/10 px-3 py-2 text-body-sm text-destructive">
        <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
        {captionState.error ||
          'Falló la generación de uno o más clips. Puedes corregirlos a mano o generar los pendientes.'}
      </p>
    );
  } else if (
    captionState?.status === CAPTION_GENERATION_STATUS.reviewRequired ||
    (captionState?.status === CAPTION_GENERATION_STATUS.ready && captionReviewBlocked)
  ) {
    notice = (
      <p role="status" className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning">
        <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
        Los candidatos todavía no son subtítulos finales. Aprueba cada clip para desbloquear el render.
      </p>
    );
  } else {
    notice = (
      <p className="text-body-sm text-fg-3">
        Guarda los rangos actuales y pulsa Generar candidatos para empezar la revisión.
      </p>
    );
  }

  let headTag: ReactNode = null;
  if (!enabled) {
    headTag = <StatusTag>DESACTIVADOS</StatusTag>;
  } else if (!sourceHasAudio) {
    headTag = <StatusTag>SIN AUDIO</StatusTag>;
  } else if (captionReviewBlocked) {
    headTag = <StatusTag tone="warning">REVISIÓN PENDIENTE</StatusTag>;
  } else {
    headTag = <StatusTag tone="success">REVISADOS</StatusTag>;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <SectionEyebrow label="SUBTÍTULOS" />
        {headTag}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Button
          type="button"
          variant={enabled ? 'default' : 'outline'}
          size="sm"
          disabled={busy}
          aria-pressed={enabled}
          onClick={() => onToggle(!enabled)}
        >
          <Captions className="size-4" aria-hidden />
          {enabled ? 'Subtítulos incrustados: activados' : 'Subtítulos incrustados: desactivados'}
        </Button>
        {enabled ? (
          <div className="flex flex-wrap items-center gap-2">
            <StatusTag>SALIDA · ESPAÑOL</StatusTag>
            {canGenerateCaptions ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={busy || captionLoading}
                loading={captionRequestBusy || captionGenerating}
                onClick={onGenerate}
              >
                {captionRequestBusy || captionGenerating ? null : <RefreshCw className="size-4" aria-hidden />}
                {captionState?.status === CAPTION_GENERATION_STATUS.none || captionState === null
                  ? 'GENERAR CANDIDATOS'
                  : 'GENERAR PENDIENTES'}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>

      {enabled ? (
        <div className="flex flex-col gap-4">
          <p className="text-body-sm text-fg-3">
            La IA genera candidatos separados del render. Revisa el texto y los tiempos de cada clip;
            FragForge no los incrusta hasta que los apruebes o confirmes que no hay voz.
          </p>

          {notice}

          {(captionState?.warnings ?? []).map((warning) => (
            <p
              key={warning}
              className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning"
            >
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              {warning}
            </p>
          ))}

          {captionState && captionState.status !== CAPTION_GENERATION_STATUS.none && !captionGenerating ? (
            <div className="flex flex-col gap-3">
              {clips.map((clip, index) => (
                <StreamCaptionReviewCard
                  key={clip.id}
                  videoSrc={videoSrc}
                  clip={clip}
                  clipNumber={index + 1}
                  candidate={(captionState.clips ?? []).find((item) => item.clip_id === clip.id)}
                  words={captionDrafts[clip.id] ?? clip.caption_words ?? []}
                  disabled={busy}
                  reviewing={reviewingClipId === clip.id}
                  onWordsChange={(words) => onWordsChange(clip.id, words)}
                  onApprove={() => onApprove(clip)}
                  onNoSpeech={() => onNoSpeech(clip)}
                />
              ))}
            </div>
          ) : null}

          {captionError ? (
            <p role="alert" className="flex items-start gap-2 border border-destructive/45 bg-destructive/10 px-3 py-2 text-body-sm text-destructive">
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              {captionError}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
