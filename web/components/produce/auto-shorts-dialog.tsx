'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Layers, RefreshCw } from 'lucide-react';
import {
  AUTO_SHORTS_DEFAULT_LIMIT,
  AUTO_SHORTS_DEFAULT_MIN_SCORE,
  AUTO_SHORTS_LEVELS,
  AUTO_SHORTS_LIMITS,
  fetchShortsBatch,
  type AutoShortsBatch,
} from '@/lib/api/auto-shorts';
import type { Play } from '@/lib/api/types';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

export type AutoShortsDialogProps = {
  matchId: string;
  plays: readonly Play[];
  disabled: boolean;
  creating: boolean;
  /** Starts one capture of these plays, each rendered as its own Short. */
  onCreate: (playIds: string[]) => void;
};

type BatchView = { status: 'loading' } | { status: 'failed' } | { status: 'ready'; batch: AutoShortsBatch };

/** Auto-Shorts: Studio picks the best play of each round over a bar and makes one Short of each. */
export function AutoShortsDialog({ matchId, plays, disabled, creating, onCreate }: AutoShortsDialogProps): ReactNode {
  const [open, setOpen] = useState(false);
  const [minScore, setMinScore] = useState(AUTO_SHORTS_DEFAULT_MIN_SCORE);
  const [limit, setLimit] = useState(AUTO_SHORTS_DEFAULT_LIMIT);
  const [attempt, setAttempt] = useState(0);
  const [view, setView] = useState<BatchView>({ status: 'loading' });

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    setView({ status: 'loading' });
    fetchShortsBatch(matchId, { minScore, limit, signal: controller.signal })
      .then((batch) => setView({ status: 'ready', batch }))
      .catch(() => {
        if (!controller.signal.aborted) setView({ status: 'failed' });
      });
    return () => controller.abort();
  }, [open, matchId, minScore, limit, attempt]);

  const byId = new Map(plays.map((play) => [play.id, play]));
  const chosen = view.status === 'ready' ? view.batch.segmentIds.filter((id) => byId.has(id)) : [];
  const level = AUTO_SHORTS_LEVELS.find((option) => option.minScore === minScore);

  return (
    <Dialog open={open} onOpenChange={(next) => !creating && setOpen(next)}>
      <DialogTrigger asChild>
        <Button type="button" size="xs" variant="outline" disabled={disabled || plays.length === 0}
          title="Hace un Short aparte de cada gran jugada, la mejor de cada ronda. Antes de crear eliges el nivel mínimo y cuántos Shorts.">
          <Layers aria-hidden />Auto-Shorts
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[85dvh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Auto-Shorts</DialogTitle>
          <DialogDescription>
            Un Short por cada gran jugada: la mejor de cada ronda que llegue al nivel elegido. CS2 las graba todas en una sola sesión y cada una sale como un Short aparte, con el estilo, la música y los textos que ya tienes elegidos.
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="flex min-w-0 flex-col gap-1.5 text-body-sm text-fg-2">
            Nivel mínimo
            <Select value={String(minScore)} onValueChange={(value) => setMinScore(Number(value))} disabled={creating}>
              <SelectTrigger className="h-10"><SelectValue /></SelectTrigger>
              <SelectContent>
                {AUTO_SHORTS_LEVELS.map((option) => <SelectItem key={option.minScore} value={String(option.minScore)}>{option.label}</SelectItem>)}
              </SelectContent>
            </Select>
          </label>
          <label className="flex min-w-0 flex-col gap-1.5 text-body-sm text-fg-2">
            Máximo de Shorts
            <Select value={String(limit)} onValueChange={(value) => setLimit(Number(value))} disabled={creating}>
              <SelectTrigger className="h-10"><SelectValue /></SelectTrigger>
              <SelectContent>
                {AUTO_SHORTS_LIMITS.map((option) => <SelectItem key={option} value={String(option)}>{option}</SelectItem>)}
              </SelectContent>
            </Select>
          </label>
        </div>
        {level ? <p className="text-meta text-fg-3">{level.hint}</p> : null}
        <section aria-label="Jugadas elegidas" aria-live="polite" className="flex min-h-24 flex-col gap-2 rounded-xl bg-surface-2 p-3">
          {view.status === 'loading' ? <p className="text-body-sm text-fg-2">Buscando jugadas…</p> : null}
          {view.status === 'failed' ? (
            <div role="alert" className="flex flex-wrap items-center gap-2 text-body-sm text-fg-2">
              <span>No se pudieron buscar las jugadas.</span>
              <Button type="button" size="xs" variant="outline" onClick={() => setAttempt((n) => n + 1)}><RefreshCw aria-hidden />Reintentar</Button>
            </div>
          ) : null}
          {view.status === 'ready' && chosen.length === 0 ? (
            <p className="text-body-sm text-fg-2">Ninguna jugada llega a este nivel. Prueba con un nivel más bajo.</p>
          ) : null}
          {chosen.length > 0 ? (
            <ol className="flex flex-col gap-1.5">
              {chosen.map((id, index) => {
                const play = byId.get(id);
                return <li key={id} className="flex min-w-0 gap-2 text-body-sm text-fg-1">
                  <span className="tabular-nums text-fg-3">{index + 1}.</span>
                  <span className="min-w-0 wrap-anywhere">{play?.label}</span>
                </li>;
              })}
            </ol>
          ) : null}
        </section>
        <DialogFooter>
          <Button type="button" variant="outline" disabled={creating} onClick={() => setOpen(false)}>Cancelar</Button>
          <Button type="button" variant="default" disabled={chosen.length === 0} loading={creating} loadingText="Preparando grabación…" onClick={() => onCreate(chosen)}>
            {chosen.length === 1 ? 'Crear 1 Short' : `Crear ${chosen.length} Shorts`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
