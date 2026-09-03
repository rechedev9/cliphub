'use client';

import { useEffect, useRef, useState } from 'react';
import { CheckCircle2, Settings2 } from 'lucide-react';
import { api } from '@/lib/api';
import type { EditConfig, Video } from '@/lib/api/types';
import { DEFAULT_EDIT_CONFIG } from '@/lib/api/reel-store';
import { constrainEditConfig, isLandscapeRecap, reelCreativeBrief } from '@/lib/reel-brief';
import { FULL_DEMO_CONTRACT, FULL_DEMO_PRESET } from '@/lib/full-demo';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { EditOptions } from '@/components/clips/edit-options';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';

/** QA review resolution: fix + re-render, or document the warnings as intentional. */
export function ReviewResolutionDialog({
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

  const lockedFullDemo = isLandscapeRecap(original);
  const editChanged = JSON.stringify(draft) !== JSON.stringify(original);
  const reviewChanged = reviewSnapshot !== null && (
    reviewSnapshot.artifactPrefix !== video.reviewArtifactPrefix ||
    JSON.stringify(reviewSnapshot.warnings) !== JSON.stringify(video.warnings ?? [])
  );

  function changeDraft(next: EditConfig) {
    if (lockedFullDemo) return;
    setDraft(constrainEditConfig(next));
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

  const musicBrief = video.songId
    ? { status: 'track' as const, title: video.songId, volumePercent: 100, gameVolumePercent: 100 }
    : { status: 'none' as const };
  const briefLines = lockedFullDemo
    ? FULL_DEMO_CONTRACT.map((row) => `${row.label}: ${row.value}`)
    : reelCreativeBrief(draft, null, musicBrief).map((item) => `${item.label}: ${item.value}`);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-3xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Resolver revisión QA</DialogTitle>
          <DialogDescription>
            {lockedFullDemo
              ? 'Inspecciona cada aviso. El contrato Full Demo es de solo lectura aquí; documenta por qué el resultado es intencional, o recaptura si hace falta cambiar el HUD.'
              : 'Inspecciona cada aviso. Corrige la edición y vuelve a renderizar, o documenta por qué el resultado es intencional.'}
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

        {lockedFullDemo ? (
          <section className="space-y-3 rounded-md border border-border p-4">
            <div>
              <h3 className="font-display text-body uppercase tracking-wide">Contrato Full Demo</h3>
              <p className="text-body-sm text-fg-3">
                16:9, rondas en vivo, HUD nativo y comms: no se pueden convertir en un Short desde este diálogo.
              </p>
            </div>
            <dl className="grid gap-1.5 text-body-sm text-fg-2 sm:grid-cols-2">
              {FULL_DEMO_CONTRACT.map((row) => (
                <div key={row.label} className="flex min-w-0 gap-1.5">
                  <dt className="shrink-0 text-fg-3">{row.label}:</dt>
                  <dd className="truncate text-fg-1">{row.value}</dd>
                </div>
              ))}
            </dl>
            <p className="text-body-sm text-fg-3">
              Preset de captura: {FULL_DEMO_PRESET.label} (no cambia sin recaptura).
            </p>
          </section>
        ) : (
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
            <EditOptions
              value={draft}
              onChange={changeDraft}
              disabled={busy !== null || reviewChanged}
            />
            <div className="rounded-md bg-surface-2 p-3">
              <p className="mb-2 font-mono text-meta uppercase tracking-wider text-primary">Brief efectivo</p>
              <ul className="grid gap-1 text-body-sm text-fg-2 sm:grid-cols-2">
                {briefLines.map((item) => <li key={item}>• {item}</li>)}
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
        )}

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
