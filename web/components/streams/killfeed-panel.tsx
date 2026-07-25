'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, Crosshair, Plus, RefreshCw, Trash2 } from 'lucide-react';
import type { KillfeedKill, NormalizedRect, StreamClipRange } from '@/lib/api/streams';
import { DEFAULT_KILLFEED_CROP, KILLFEED_MIN_CROP_SIZE, formatStreamTimestamp } from '@/lib/streams/plan';
import { StatusTag } from '@/components/studio/status-tag';
import { Button } from '@/components/ui/button';
import { CropPicker } from '@/components/streams/crop-picker';
import { KillfeedKillsEditor } from '@/components/streams/killfeed-kills-editor';
import { StreamAnalysisProgress } from '@/components/streams/analysis-progress';

function cueKey(clipId: string, cue: number): string {
  return `${clipId}@${cue}`;
}

/**
 * Clean-killfeed controls: the source crop, the automatic per-frame analysis,
 * and the manual cue corrections with their confirmed kills.
 *
 * The analysis is the slowest thing on this screen, so its state is carried by
 * a status tag and a real progress region rather than by a sentence fragment.
 * Nothing here invents a detection: an unanalysed clip says so.
 */
export function StreamKillfeedPanel({
  enabled,
  crop,
  clips,
  weapons,
  busy,
  analyzing,
  clipsValid,
  hasAnalysis,
  analysisApplied,
  detectedEvents,
  error,
  warnings,
  needsReanalysis,
  readNotice,
  readingCueKey,
  readErrors,
  previewSeconds,
  sourceDuration,
  canAddCue,
  cueStatus,
  onToggle,
  onAnalyze,
  onCancelWait,
  onCropChange,
  onPreviewSecondsChange,
  onAddCue,
  onRemoveCue,
  onCueKillsChange,
  onReadCue,
}: {
  enabled: boolean;
  crop?: NormalizedRect;
  clips: StreamClipRange[];
  weapons: string[];
  busy: boolean;
  analyzing: boolean;
  clipsValid: boolean;
  hasAnalysis: boolean;
  analysisApplied: boolean;
  detectedEvents: number;
  error: string | null;
  warnings?: string[];
  needsReanalysis: boolean;
  readNotice: string | null;
  readingCueKey: string | null;
  readErrors: Record<string, string>;
  previewSeconds: number;
  sourceDuration: number;
  canAddCue: boolean;
  cueStatus: string;
  onToggle: (enabled: boolean) => void;
  onAnalyze: () => void;
  onCancelWait: () => void;
  onCropChange: (rect: NormalizedRect) => void;
  onPreviewSecondsChange: (seconds: number) => void;
  onAddCue: () => void;
  onRemoveCue: (clipId: string, cue: number) => void;
  onCueKillsChange: (clipId: string, cue: number, kills: KillfeedKill[]) => void;
  onReadCue: (clip: StreamClipRange, cue: number) => void;
}): ReactNode {
  let analysisTag: ReactNode = null;
  if (analyzing) {
    analysisTag = <StatusTag tone="stream" dot>ANALIZANDO</StatusTag>;
  } else if (hasAnalysis && analysisApplied) {
    analysisTag = (
      <StatusTag tone="success" icon={Crosshair}>
        {detectedEvents} {detectedEvents === 1 ? 'EVENTO' : 'EVENTOS'}
      </StatusTag>
    );
  } else if (enabled) {
    analysisTag = <StatusTag tone="warning">SIN ANALIZAR</StatusTag>;
  }

  return (
    <section aria-labelledby="killfeed-clean-title" className="flex flex-col gap-4 border-t border-border pt-5">
      <div className="flex flex-col gap-3 @[36rem]/editor:flex-row @[36rem]/editor:items-start @[36rem]/editor:justify-between">
        <div className="flex max-w-2xl flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-2.5">
            <h3 id="killfeed-clean-title" className="font-display text-body-lg font-bold text-fg-1">
              Killfeed limpia (opcional)
            </h3>
            {analysisTag}
          </div>
          <p id="killfeed-clean-description" className="text-body-sm text-fg-2">
            FragForge recorre todos los clips, localiza cada nacimiento por PTS del vídeo y coordina
            automáticamente su captura. La edición manual queda disponible como corrección.
          </p>
        </div>
        <Button
          id="killfeed-clean-toggle"
          type="button"
          variant={enabled ? 'stream' : 'outline'}
          size="sm"
          disabled={busy}
          aria-pressed={enabled}
          aria-expanded={enabled}
          aria-controls="killfeed-clean-controls"
          aria-describedby="killfeed-clean-description"
          onClick={() => onToggle(!enabled)}
          className="shrink-0"
        >
          {enabled ? 'Killfeed: activada' : 'Activar killfeed limpia'}
        </Button>
      </div>

      {enabled ? (
        <div id="killfeed-clean-controls" className="flex flex-col gap-4">
          <p className="text-body-sm text-fg-3">
            Ajusta el recorte para que cubra holgadamente el área de la killfeed. Tras dejar de moverlo,
            FragForge vuelve a analizar los rangos automáticamente. El cue se guarda con el primer
            fotograma fuente verificable; un fotograma posterior se usa solo para leer o congelar el aviso.
          </p>

          <div className="flex flex-wrap items-start gap-3">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy || !clipsValid}
              loading={analyzing}
              onClick={onAnalyze}
            >
              {analyzing ? null : <RefreshCw className="size-4" />}
              {hasAnalysis ? 'REANALIZAR KILLFEED' : 'ANALIZAR KILLFEED'}
            </Button>
            {analyzing ? (
              <StreamAnalysisProgress label="Analizando clips por fotograma" onCancel={onCancelWait} />
            ) : null}
          </div>

          {error ? (
            <p role="alert" className="flex items-start gap-2 border border-destructive/45 bg-destructive/10 px-3 py-2 text-body-sm text-destructive">
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              {error}
            </p>
          ) : null}

          {needsReanalysis ? (
            <p role="alert" className="flex items-start gap-2 border border-destructive/45 bg-destructive/10 px-3 py-2 text-body-sm text-destructive">
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              Las capturas exactas ya no están disponibles. Pulsa REANALIZAR KILLFEED antes de crear los Shorts otra vez.
            </p>
          ) : null}

          {(warnings ?? []).map((warning) => (
            <p
              key={warning}
              className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning"
            >
              <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
              {warning}
            </p>
          ))}

          {readNotice ? (
            <p role="status" className="text-body-sm text-stream-text">
              {readNotice}
            </p>
          ) : null}

          <CropPicker
            rect={crop ?? DEFAULT_KILLFEED_CROP}
            onChange={onCropChange}
            kind="killfeed"
            frameSeconds={previewSeconds}
            durationSeconds={sourceDuration}
            onFrameSecondsChange={onPreviewSecondsChange}
            showScrubber
            minWidth={KILLFEED_MIN_CROP_SIZE}
            minHeight={KILLFEED_MIN_CROP_SIZE}
            disabled={busy}
          />

          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy || !canAddCue}
              aria-describedby="killfeed-cue-status"
              onClick={onAddCue}
            >
              <Plus className="size-4" aria-hidden />
              Añadir corrección en {formatStreamTimestamp(previewSeconds)}
            </Button>
            <p id="killfeed-cue-status" className="min-w-48 flex-1 text-body-sm text-fg-3">
              {cueStatus}
            </p>
          </div>

          <div className="flex flex-col divide-y divide-border border-y border-border">
            {clips.map((clip, index) => {
              const cues = clip.killfeed_seconds ?? [];
              const clipTitle = clip.title?.trim();
              const clipLabel = clipTitle ? `Clip ${index + 1}: ${clipTitle}` : `Clip ${index + 1}`;
              const headingId = `killfeed-cues-${clip.id}`;
              return (
                <section key={clip.id} aria-labelledby={headingId} className="flex flex-col gap-3 py-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <h4
                      id={headingId}
                      className="font-display text-label font-bold uppercase tracking-wide text-fg-1"
                    >
                      {clipLabel}
                    </h4>
                    <span className="font-mono text-meta tabular-nums text-fg-3">
                      {formatStreamTimestamp(clip.start_seconds)} → {formatStreamTimestamp(clip.end_seconds)}
                    </span>
                  </div>

                  {cues.length > 0 ? (
                    <ul className="flex flex-col gap-3" aria-label={`Marcas de ${clipLabel}`}>
                      {cues.map((cue, cueIndex) => {
                        const key = cueKey(clip.id, cue);
                        const kills = clip.killfeed_kills?.[cueIndex] ?? [];
                        return (
                          <li
                            key={`${clip.id}-${cue}`}
                            className="flex flex-col gap-3 border border-stream/30 bg-stream/[0.05] p-3 shadow-[var(--elev-0)]"
                          >
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <div className="flex flex-wrap items-center gap-2.5">
                                <span aria-hidden className="font-mono text-meta tabular-nums text-fg-4">
                                  {String(cueIndex + 1).padStart(2, '0')}
                                </span>
                                <button
                                  type="button"
                                  disabled={busy}
                                  aria-label={`Mostrar la marca ${formatStreamTimestamp(cue)} de ${clipLabel}`}
                                  onClick={() => onPreviewSecondsChange(cue)}
                                  className="inline-flex min-h-10 items-center border border-stream/45 bg-stream/10 px-3 font-mono text-label tabular-nums text-stream-text transition-colors duration-(--dur-fast) ease-standard hover:bg-stream/20 hover:text-stream focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50"
                                >
                                  {formatStreamTimestamp(cue)}
                                </button>
                                <StatusTag tone={kills.length > 0 ? 'stream' : 'neutral'}>
                                  {kills.length > 0
                                    ? `${kills.length} ${kills.length === 1 ? 'KILL' : 'KILLS'}`
                                    : 'CONGELA EL RECORTE'}
                                </StatusTag>
                              </div>
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon-sm"
                                disabled={busy}
                                aria-label={`Eliminar la marca ${formatStreamTimestamp(cue)} de ${clipLabel}`}
                                onClick={() => onRemoveCue(clip.id, cue)}
                              >
                                <Trash2 className="size-4" aria-hidden />
                              </Button>
                            </div>
                            <KillfeedKillsEditor
                              kills={kills}
                              weapons={weapons}
                              reading={readingCueKey === key}
                              readError={readErrors[key] ?? null}
                              disabled={busy}
                              onChange={(next) => onCueKillsChange(clip.id, cue, next)}
                              onReadWithAI={() => onReadCue(clip, cue)}
                            />
                          </li>
                        );
                      })}
                    </ul>
                  ) : (
                    <p className="border border-dashed border-border px-3 py-2.5 text-body-sm text-fg-3">
                      Aún no se detectaron eventos de killfeed en este clip.
                    </p>
                  )}
                </section>
              );
            })}
          </div>

          <p className="text-body-sm text-fg-3">
            Los eventos automáticos se ordenan por PTS. Cambiar crop o rangos invalida el análisis anterior y lanza uno nuevo.
          </p>
        </div>
      ) : (
        <p className="text-body-sm text-fg-3">
          Desactivada: el render conserva exactamente el flujo actual, sin recorte ni avisos superpuestos.
        </p>
      )}
    </section>
  );
}
