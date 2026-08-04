'use client';

import { useEffect, useRef, useState } from 'react';
import { AlertTriangle, CheckCircle2, Clock, Download, Eye, Settings2, Share2, Youtube } from 'lucide-react';
import { toast } from 'sonner';
import { api } from '@/lib/api';
import type { EditConfig, Video } from '@/lib/api/types';
import { DEFAULT_EDIT_CONFIG } from '@/lib/api/reel-store';
import { writeClipboardText } from '@/lib/clipboard-write';
import { formatCountdown } from '@/lib/format';
import { downloadPublishMP4 } from '@/lib/publish-actions';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { StatusTag } from '@/components/studio/status-tag';
import { DeleteVideoButton } from '@/components/videos/delete-video-button';
import { PublishAssistantDialog } from '@/components/videos/publish-assistant-dialog';
import { ReelCard, reelFormatLabel } from '@/components/videos/reel-card';
import { EditOptions } from '@/components/clips/edit-options';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { CoverImage } from '@/components/studio/cover-image';

/**
 * A finished, downloadable reel. The card is the shared `ReelCard` in its payoff
 * state: raised off the grid, cyan-edged, its cover finally rendered at the
 * format it was actually made in, and its stage track filled to LISTO.
 *
 * Hovering (or focusing, or touching) the frame surfaces Ver/Compartir. There is
 * no thumbnail-duration data, so the corner tag shows the render format and
 * nothing else — the shape of the frame now carries most of that information
 * anyway. Ver plays the reel inline in a dialog.
 */
export function ReadyCard({
  video,
  onChange,
}: {
  video: Video;
  onChange?: () => void;
}) {
  const [publishOpen, setPublishOpen] = useState(false);
  const [playerOpen, setPlayerOpen] = useState(false);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [coverBusy, setCoverBusy] = useState(false);
  const [coverError, setCoverError] = useState<string | null>(null);
  const reviewRequired = video.status === 'review_required';
  const coverStrategy = video.editConfig?.coverStrategy ?? 'generated-gameplay';
  const coverCandidates = video.coverCandidates ?? [];
  const needsCoverGate =
    !reviewRequired &&
    coverStrategy !== 'no-cover' &&
    coverCandidates.length > 0;
  const coverApproved =
    !needsCoverGate ||
    (video.selectedCoverName != null && coverCandidates.includes(video.selectedCoverName));

  const handleDownload = () => {
    if (!video.downloadUrl) return;
    downloadPublishMP4(video.downloadUrl, video.title);
  };

  const handleSelectCover = async (coverName: string) => {
    if (coverBusy) return;
    setCoverBusy(true);
    setCoverError(null);
    try {
      await api.selectVideoCover(video.id, coverName);
      onChange?.();
    } catch (err) {
      setCoverError(err instanceof Error ? err.message : 'No se pudo seleccionar la portada.');
    } finally {
      setCoverBusy(false);
    }
  };

  // In cloud mode the reel's media is a DOM object URL (blob:) fetched through the
  // Bearer-gated loopback: it lives and dies with this tab, so there is no
  // persistent URL to share. Hide Share entirely there rather than copy a link
  // that dies with the tab. Download and inline playback still work with blob:.
  const canShare = video.downloadUrl != null && !video.downloadUrl.startsWith('blob:');

  const handleShare = async () => {
    if (!video.downloadUrl) return;
    const url = new URL(video.downloadUrl, window.location.origin).href;
    try {
      if (typeof navigator !== 'undefined' && navigator.share) {
        await navigator.share({ title: video.title, url });
        return;
      }
    } catch {
      // user dismissed the share sheet, or it failed — fall through to copy.
    }
    try {
      await writeClipboardText(url);
      toast('Enlace copiado al portapapeles.');
    } catch {
      toast('No se pudo copiar el enlace.');
    }
  };

  const matchMeta = video.score ? `${video.map} · ${video.score}` : video.map;
  const meta = video.targetName ? `POV ${video.targetName} · ${matchMeta}` : matchMeta;
  const formatBadge = reelFormatLabel(video.editConfig);

  return (
    <>
      <ReelCard
        video={video}
        tone="primary"
        raised
        scrim
        badge={formatBadge ? <StatusTag tone="primary">{formatBadge}</StatusTag> : undefined}
        frameActions={
          <>
            <Button
              type="button"
              variant="outline-primary"
              size="sm"
              onClick={() => video.downloadUrl && setPlayerOpen(true)}
              disabled={!video.downloadUrl}
            >
              <Eye className="size-4" aria-hidden /> Ver
            </Button>
            {canShare ? (
              <Button type="button" variant="outline" size="sm" onClick={handleShare}>
                <Share2 className="size-4" aria-hidden /> Compartir
              </Button>
            ) : null}
          </>
        }
        footer={
          <div className="flex flex-col gap-2 p-4">
            {needsCoverGate ? (
              <section
                className="space-y-2 border border-primary/25 bg-primary/[0.04] p-3"
                aria-labelledby={`cover-gate-${video.id}`}
              >
                <p
                  id={`cover-gate-${video.id}`}
                  className="font-mono text-meta uppercase tracking-wider text-primary"
                >
                  Portada · elige candidata
                </p>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                  {coverCandidates.map((name) => {
                    const selected = video.selectedCoverName === name;
                    const src =
                      video.jobId && video.variant
                        ? `/api/demos/${video.jobId}/renders/${video.variant}/covers/${name}`
                        : undefined;
                    return (
                      <button
                        key={name}
                        type="button"
                        disabled={coverBusy}
                        onClick={() => void handleSelectCover(name)}
                        className={
                          selected
                            ? 'relative aspect-video overflow-hidden border-2 border-primary focus-visible:outline-2 focus-visible:outline-ring'
                            : 'relative aspect-video overflow-hidden border border-border-strong opacity-90 hover:border-primary/60 focus-visible:outline-2 focus-visible:outline-ring'
                        }
                        aria-pressed={selected}
                        aria-label={`Seleccionar portada ${name}`}
                      >
                        <CoverImage src={src} className="absolute inset-0 size-full object-cover" />
                      </button>
                    );
                  })}
                </div>
                {!coverApproved ? (
                  <p className="text-body-sm text-fg-2">
                    Confirma una portada antes de marcar el pack listo para subir.
                  </p>
                ) : (
                  <p className="text-body-sm text-fg-2">Portada aprobada.</p>
                )}
                {coverError ? (
                  <p role="alert" className="text-body-sm text-destructive">{coverError}</p>
                ) : null}
              </section>
            ) : null}
            {/* Full width, and allowed to wrap rather than being starved into a
                112px column by a three-track footer grid. */}
            {reviewRequired ? (
              <Button
                type="button"
                variant="warning"
                className="h-auto min-h-11 w-full whitespace-normal px-3 py-2.5 text-center leading-tight"
                onClick={() => setReviewOpen(true)}
              >
                <Settings2 className="size-4" aria-hidden /> RESOLVER REVISIÓN QA
              </Button>
            ) : (
              <Button
                type="button"
                variant="hero"
                className="h-auto min-h-11 w-full whitespace-normal px-3 py-2.5 text-center leading-tight"
                onClick={() => setPublishOpen(true)}
                disabled={!coverApproved}
              >
                <Youtube className="size-4" aria-hidden /> PREPARAR PUBLICACIÓN
              </Button>
            )}
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline-primary"
                size="sm"
                className="flex-1"
                onClick={handleDownload}
                disabled={!video.downloadUrl || !coverApproved}
              >
                <Download className="size-4" aria-hidden /> MP4
              </Button>
              <DeleteVideoButton video={video} onDeleted={() => onChange?.()} />
            </div>
          </div>
        }
      >
        <div className="flex flex-wrap items-center gap-2">
          {reviewRequired ? (
            <StatusTag tone="warning" icon={AlertTriangle}>
              Revisión necesaria
            </StatusTag>
          ) : (
            <StatusTag tone="success" dot>
              Listo
            </StatusTag>
          )}
          {video.availableForSec !== undefined ? (
            <StatusTag tone="warning" icon={Clock}>
              caduca en <span className="tabular-nums">{formatCountdown(video.availableForSec)}</span>
            </StatusTag>
          ) : null}
        </div>
        {reviewRequired && video.warnings?.length ? (
          <ul className="space-y-1 text-body-sm text-fg-2">
            {video.warnings.map((warning) => (
              <li key={warning}>• {warning}</li>
            ))}
          </ul>
        ) : null}
      </ReelCard>

      <Dialog open={playerOpen} onOpenChange={setPlayerOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="truncate">{video.title}</DialogTitle>
            <DialogDescription className="font-mono tabular-nums">{meta}</DialogDescription>
          </DialogHeader>
          {video.downloadUrl ? (
            <video
              src={video.downloadUrl}
              controls
              autoPlay
              playsInline
              preload="metadata"
              className="mx-auto max-h-[72vh] w-auto rounded-lg bg-surface-0"
            />
          ) : null}
        </DialogContent>
      </Dialog>

      {!reviewRequired ? (
        <PublishAssistantDialog open={publishOpen} video={video} onOpenChange={setPublishOpen} />
      ) : null}
      {reviewRequired ? (
        <ReviewResolutionDialog
          open={reviewOpen}
          video={video}
          onOpenChange={setReviewOpen}
          onResolved={() => onChange?.()}
        />
      ) : null}
    </>
  );
}

function ReviewResolutionDialog({
  open,
  video,
  onOpenChange,
  onResolved,
}: {
  open: boolean;
  video: Video;
  onOpenChange: (open: boolean) => void;
  onResolved: () => void;
}) {
  const original = video.editConfig ?? DEFAULT_EDIT_CONFIG;
  const [draft, setDraft] = useState<EditConfig>(original);
  const [reviewSnapshot, setReviewSnapshot] = useState<{
    artifactPrefix: string;
    warnings: string[];
  } | null>(null);
  const [briefApproved, setBriefApproved] = useState(false);
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState<'rerender' | 'accept' | null>(null);
  const [error, setError] = useState<string | null>(null);

  const wasOpen = useRef(false);
  useEffect(() => {
    if (open && !wasOpen.current) {
      setDraft(original);
      setReviewSnapshot(
        video.reviewArtifactPrefix && video.warnings
          ? {
              artifactPrefix: video.reviewArtifactPrefix,
              warnings: [...video.warnings],
            }
          : null,
      );
      setBriefApproved(false);
      setNote('');
      setError(null);
    } else if (!open) {
      setReviewSnapshot(null);
    }
    wasOpen.current = open;
  }, [open, original, video.reviewArtifactPrefix, video.warnings]);

  const editChanged = JSON.stringify(draft) !== JSON.stringify(original);
  const reviewChanged = reviewSnapshot !== null && (
    reviewSnapshot.artifactPrefix !== video.reviewArtifactPrefix ||
    JSON.stringify(reviewSnapshot.warnings) !== JSON.stringify(video.warnings ?? [])
  );

  function changeDraft(next: EditConfig) {
    setDraft(next);
    setBriefApproved(false);
  }

  async function resolveReview(kind: 'rerender' | 'accept') {
    if (busy) return;
    if (!reviewSnapshot || reviewChanged) {
      setError('La revisión cambió o todavía no tiene un identificador durable. Actualiza la biblioteca e inspecciona los avisos actuales.');
      onResolved();
      return;
    }
    const expectedArtifactPrefix = reviewSnapshot.artifactPrefix;
    const expectedWarnings = reviewSnapshot.warnings;
    setBusy(kind);
    setError(null);
    try {
      await api.resolveVideoReview(
        video.id,
        kind === 'rerender'
          ? { kind, editConfig: draft, expectedArtifactPrefix, expectedWarnings }
          : { kind, note, expectedArtifactPrefix, expectedWarnings },
      );
      onOpenChange(false);
      onResolved();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo resolver la revisión.');
      onResolved();
    } finally {
      setBusy(null);
    }
  }

  const brief = [
    `Formato: ${draft.format}`,
    `HUD/captura: ${video.variant ?? 'viral-60-clean'} (no cambia sin recaptura)`,
    `Efecto de kill: ${draft.killEffect}`,
    `Transición: ${draft.transition}`,
    `Contador: ${draft.killCounter ? 'sí' : 'no'}`,
    `Título automático: ${draft.hookText ? 'sí' : 'no'}`,
    `Intro / outro: ${draft.intro ? 'sí' : 'no'} / ${draft.outro ? 'sí' : 'no'}`,
    `Música: ${video.songId ?? 'sin música'}`,
    `Portada: ${draft.coverStrategy}`,
  ];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Resolver revisión QA</DialogTitle>
          <DialogDescription>
            Inspecciona cada aviso. Corrige la edición y vuelve a renderizar, o documenta por qué el resultado es intencional.
          </DialogDescription>
        </DialogHeader>

        <div role="alert" className="rounded-md border border-warning/40 bg-warning/10 p-3">
          <p className="mb-2 font-mono text-meta uppercase tracking-wider text-warning">Avisos actuales</p>
          <ul className="space-y-1 text-body-sm text-fg-2">
            {(reviewSnapshot?.warnings ?? []).map((warning) => <li key={warning}>• {warning}</li>)}
          </ul>
          {reviewChanged ? (
            <p className="mt-2 text-body-sm text-warning">
              Hay una revisión más reciente. Cierra este diálogo y vuelve a inspeccionarla.
            </p>
          ) : null}
        </div>

        <section className="space-y-4 rounded-md border border-border p-4">
          <div>
            <h3 className="font-display text-body uppercase tracking-wide">Corregir y volver a renderizar</h3>
            <p className="text-body-sm text-fg-3">Debes cambiar al menos una opción y aprobar el brief completo.</p>
          </div>
          <div>
            <p className="mb-2 font-mono text-meta uppercase tracking-wider text-fg-3">Formato</p>
            <ToggleGroup
              type="single"
              value={draft.format}
              onValueChange={(format) => format && changeDraft({ ...draft, format: format as EditConfig['format'] })}
              disabled={busy !== null || reviewChanged}
              variant="outline"
            >
              <ToggleGroupItem value="short-9x16">Vertical 9:16</ToggleGroupItem>
              <ToggleGroupItem value="landscape-16x9">Horizontal 16:9</ToggleGroupItem>
            </ToggleGroup>
          </div>
          <EditOptions value={draft} onChange={changeDraft} disabled={busy !== null || reviewChanged} />
          <div className="rounded-md bg-surface-2 p-3">
            <p className="mb-2 font-mono text-meta uppercase tracking-wider text-primary">Brief efectivo</p>
            <ul className="grid gap-1 text-body-sm text-fg-2 sm:grid-cols-2">
              {brief.map((item) => <li key={item}>• {item}</li>)}
            </ul>
          </div>
          <label className="flex items-start gap-2 text-body-sm text-fg-2">
            <input
              type="checkbox"
              className="mt-0.5 size-4 accent-primary"
              checked={briefApproved}
              onChange={(event) => setBriefApproved(event.target.checked)}
              disabled={busy !== null || reviewChanged}
            />
            Apruebo este brief exacto para el nuevo render.
          </label>
          <Button
            type="button"
            variant="hero"
            className="w-full"
            disabled={!editChanged || !briefApproved || reviewChanged}
            loading={busy === 'rerender'} loadingText="RENDERIZANDO DE NUEVO…"
            onClick={() => void resolveReview('rerender')}
          >
            <Settings2 className="size-4" aria-hidden /> VOLVER A RENDERIZAR
          </Button>
        </section>

        <section className="space-y-3 rounded-md border border-border p-4">
          <div>
            <h3 className="font-display text-body uppercase tracking-wide">Aceptar como intencional</h3>
            <p className="text-body-sm text-fg-3">
              La nota queda ligada a esta revisión y a estos avisos; un render nuevo exigirá otra revisión.
            </p>
          </div>
          <label htmlFor="review-note" className="font-mono text-meta uppercase tracking-wider text-fg-2">
            Motivo de aceptación
          </label>
          <textarea
            id="review-note"
            value={note}
            maxLength={1000}
            disabled={busy !== null || reviewChanged}
            onChange={(event) => setNote(event.target.value)}
            className="min-h-24 w-full resize-y rounded-md border border-border-strong bg-surface-2 px-3 py-2 text-body-sm text-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            placeholder="Ej.: la pausa de 0,4 s es intencional y coincide con el beat final."
          />
          <Button
            type="button"
            variant="outline-primary"
            className="w-full"
            disabled={!note.trim() || reviewChanged}
            loading={busy === 'accept'} loadingText="DOCUMENTANDO REVISIÓN…"
            onClick={() => void resolveReview('accept')}
          >
            <CheckCircle2 className="size-4" aria-hidden /> DOCUMENTAR Y MARCAR LISTO
          </Button>
        </section>

        {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      </DialogContent>
    </Dialog>
  );
}
