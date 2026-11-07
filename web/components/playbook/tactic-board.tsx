'use client';

import { useMemo, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { BookmarkPlus, Minus, Play, Plus, X } from 'lucide-react';
import { createLibraryPlay } from '@/lib/api/library';
import { TACTIC_KINDS, type PlaybookTactic, type TacticKind, type TeamPlaybook } from '@/lib/api/playbook';
import type { LibraryPlay } from '@/lib/api/stratbook';
import {
  AWP_START_SECONDS,
  comboIsEmpty,
  comboMode,
  comboSentence,
  playFromCombo,
  tacticKindHint,
  tacticKindTitle,
  tacticLabel,
  withoutLabel,
  type ComboMode,
} from '@/lib/analysis/playbook-combo';
import { evidenceRefs, roundRefs, winPct } from '@/lib/analysis/playbook';
import { replayHref } from '@/lib/analysis/replay-link';
import { playHref } from '@/lib/clips/routes';
import { MAX_PLAY_EXAMPLES } from '@/lib/stratbook/plays';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { playbookReplayTitle } from './playbook-summary';
import { ZoneMap } from './zone-map';

type LabelOf = (key: string) => string;

function roundsText(n: number): string {
  return `${n} ${n === 1 ? 'ronda' : 'rondas'}`;
}

/** The combination as chips, its rounds and what to do with them. */
export function ComboBar({
  teamId,
  view,
  labelOf,
  onToggle,
  onClear,
}: {
  teamId: string;
  view: TeamPlaybook;
  labelOf: LabelOf;
  onToggle: (key: string, mode: ComboMode) => void;
  onClear: () => void;
}): ReactNode {
  const [saving, setSaving] = useState(false);
  const { combo } = view;
  if (comboIsEmpty(combo)) {
    return (
      <p className="text-body-sm text-fg-2">
        Pulsa <Plus aria-label="más" className="inline size-3.5 align-[-2px]" /> en cualquier fila para quedarte solo con las rondas que la cumplen, o{' '}
        <Minus aria-label="menos" className="inline size-3.5 align-[-2px]" /> para quitarlas. Todo el playbook se recuenta con la combinación.
      </p>
    );
  }
  const sentence = comboSentence(combo, labelOf);
  const won = winPct(view.rounds);
  const chips = [...combo.with.map((key) => ({ key, mode: 'with' as const })), ...combo.without.map((key) => ({ key, mode: 'without' as const }))];
  return (
    <div className="flex flex-col gap-3">
      <ul aria-label="Combinación" className="flex flex-wrap gap-2">
        {chips.map(({ key, mode }) => (
          <li key={key}>
            <button
              type="button"
              onClick={() => onToggle(key, mode)}
              aria-label={`Quitar «${mode === 'with' ? labelOf(key) : withoutLabel(labelOf(key))}» de la combinación`}
              className={cn(
                'inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-meta font-medium outline-none transition-[background-color] focus-visible:ring-[3px] focus-visible:ring-ring/50',
                mode === 'with' ? 'border-primary/50 bg-primary/15 text-fg-1 hover:bg-primary/25' : 'border-destructive/50 bg-destructive/10 text-fg-1 hover:bg-destructive/20',
              )}
            >
              {mode === 'with' ? <Plus aria-hidden className="size-3" /> : <Minus aria-hidden className="size-3" />}
              {mode === 'with' ? labelOf(key) : withoutLabel(labelOf(key))}
              <X aria-hidden className="size-3 text-fg-3" />
            </button>
          </li>
        ))}
      </ul>
      <div className="flex flex-wrap items-center gap-3">
        <p aria-live="polite" className="text-body-sm text-fg-1">
          <span className="tabular-nums">{roundsText(view.round_count)}</span>
          {won === null ? '' : `, gana el ${won} %`}
        </p>
        <div className="ml-auto flex flex-wrap items-center gap-2">
          <Button type="button" variant="ghost" size="xs" onClick={onClear}>
            <X aria-hidden />
            Quitar todo
          </Button>
          <Button type="button" variant="outline" size="sm" disabled={view.round_count === 0} onClick={() => setSaving(true)}>
            <BookmarkPlus aria-hidden />
            Guardar en la Libreta
          </Button>
          {view.round_count > 0 ? (
            <Button asChild size="sm">
              <Link href={replayHref(roundRefs(view.rounds), playbookReplayTitle(view, sentence))}>
                <Play aria-hidden />
                Reproducir {roundsText(view.round_count)}
              </Link>
            </Button>
          ) : (
            <Button type="button" size="sm" disabled>
              <Play aria-hidden />
              Reproducir 0 rondas
            </Button>
          )}
        </div>
      </div>
      {saving ? <SaveComboDialog teamId={teamId} view={view} sentence={sentence} onClose={() => setSaving(false)} /> : null}
    </div>
  );
}

function TacticRow({
  tactic,
  mode,
  view,
  onToggle,
}: {
  tactic: PlaybookTactic;
  mode: ComboMode | null;
  view: TeamPlaybook;
  onToggle: (key: string, mode: ComboMode) => void;
}): ReactNode {
  const label = tacticLabel(tactic);
  const empty = tactic.rounds === 0;
  return (
    <li
      className={cn(
        'flex min-h-10 shrink-0 flex-wrap items-center gap-x-2 gap-y-1 rounded-sm px-2 py-1.5 sm:flex-nowrap sm:py-0',
        mode === 'with' && 'bg-primary/15',
        mode === 'without' && 'bg-destructive/10',
      )}
    >
      <span className={cn('min-w-0 basis-full truncate text-body-sm sm:flex-1 sm:basis-0', mode === 'without' ? 'text-fg-3 line-through' : 'text-fg-1')} title={label}>
        {label}
      </span>
      <span className="ml-auto shrink-0 text-meta text-fg-2 tabular-nums sm:ml-0" title={`${roundsText(tactic.rounds)}${empty ? '' : `, gana el ${Math.round(tactic.win_rate.pct)} %`}`}>
        {tactic.rounds}
        {empty ? '' : ` · gana ${Math.round(tactic.win_rate.pct)} %`}
      </span>
      <Button
        type="button"
        variant={mode === 'without' ? 'secondary' : 'ghost'}
        size="icon-xs"
        aria-pressed={mode === 'without'}
        aria-label={`Quitar las rondas con ${label}`}
        onClick={() => onToggle(tactic.key, 'without')}
      >
        <Minus aria-hidden />
      </Button>
      <Button
        type="button"
        variant={mode === 'with' ? 'secondary' : 'ghost'}
        size="icon-xs"
        aria-pressed={mode === 'with'}
        aria-label={`Quedarse con las rondas con ${label}`}
        onClick={() => onToggle(tactic.key, 'with')}
      >
        <Plus aria-hidden />
      </Button>
      {empty ? (
        <span aria-hidden className="size-8 shrink-0" />
      ) : (
        <Button asChild variant="ghost" size="icon-xs">
          <Link href={replayHref(evidenceRefs(tactic.refs), playbookReplayTitle(view, label))} aria-label={`Reproducir ${roundsText(tactic.rounds)} con ${label}`}>
            <Play aria-hidden />
          </Link>
        </Button>
      )}
    </li>
  );
}

/** Each kind's rows: the counted tactics, then those of the combination that no kept round shows. */
function rowsByKind(view: TeamPlaybook, all: ReadonlyMap<string, PlaybookTactic>): Map<TacticKind, PlaybookTactic[]> {
  const out = new Map<TacticKind, PlaybookTactic[]>();
  const shown = new Set(view.tactics.map((tactic) => tactic.key));
  const zero = [...view.combo.with, ...view.combo.without].flatMap((key) => {
    const base = all.get(key);
    if (base === undefined || shown.has(key)) return [];
    return [{ ...base, rounds: 0, refs: [], win_rate: { ...base.win_rate, count: 0, total: 0, pct: 0 } }];
  });
  for (const tactic of [...view.tactics, ...zero]) out.set(tactic.kind, [...(out.get(tactic.kind) ?? []), tactic]);
  return out;
}

/**
 * The tactics of the side as cards, one per kind, each row with − and + to
 * build the combination and ▶ to replay its rounds.
 */
export function TacticBoard({
  teamId,
  playbook,
  view,
  onToggle,
}: {
  teamId: string;
  playbook: TeamPlaybook;
  view: TeamPlaybook;
  onToggle: (key: string, mode: ComboMode) => void;
}): ReactNode {
  const all = useMemo(() => new Map(playbook.tactics.map((tactic) => [tactic.key, tactic])), [playbook.tactics]);
  const rows = rowsByKind(view, all);
  const kinds = TACTIC_KINDS.filter((kind) => playbook.tactics.some((tactic) => tactic.kind === kind));
  const zoneLabels = useMemo(() => new Map(playbook.zones.map((zone) => [zone.zone, zone.label])), [playbook.zones]);
  const chosenZones = view.combo.with.flatMap((key) => (key.startsWith('zone:') ? [key.slice('zone:'.length)] : []));
  return (
    <div className="grid gap-3 lg:grid-cols-2 min-[110rem]:grid-cols-3">
      {kinds.map((kind) => {
        const list = rows.get(kind) ?? [];
        const hint = tacticKindHint(kind, AWP_START_SECONDS);
        return (
          <section key={kind} aria-label={tacticKindTitle(kind)} className="flex min-w-0 flex-col gap-2 rounded-md border border-border bg-surface-2 p-3">
            <h3 className="flex items-baseline gap-2 text-body-sm font-semibold text-fg-1">
              {tacticKindTitle(kind)}
              <span className="text-meta font-normal text-fg-3 tabular-nums">{list.length}</span>
            </h3>
            {hint === '' ? null : <p className="text-meta text-fg-3">{hint}</p>}
            {kind === 'zone' && playbook.zones.length > 0 ? (
              <ZoneMap teamId={teamId} map={playbook.map} zones={playbook.zones.map((zone) => zone.zone)} labels={zoneLabels} selected={chosenZones} />
            ) : null}
            {list.length === 0 ? (
              <p className="text-meta text-fg-3">Ninguna ronda de la combinación.</p>
            ) : (
              <ul className="flex max-h-72 flex-col gap-0.5 overflow-y-auto">
                {list.map((tactic) => (
                  <TacticRow key={tactic.key} tactic={tactic} mode={comboMode(view.combo, tactic.key)} view={view} onToggle={onToggle} />
                ))}
              </ul>
            )}
          </section>
        );
      })}
    </div>
  );
}

function SaveComboDialog({
  teamId,
  view,
  sentence,
  onClose,
}: {
  teamId: string;
  view: TeamPlaybook;
  sentence: string;
  onClose: () => void;
}): ReactNode {
  const [title, setTitle] = useState(`${sentence} (${view.team_name})`.slice(0, 120));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<LibraryPlay | null>(null);
  const kept = Math.min(view.round_count, MAX_PLAY_EXAMPLES);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      setSaved(await createLibraryPlay(playFromCombo(view, { teamId, title, sentence })));
    } catch {
      setError('No se pudo guardar la combinación. Comprueba que ClipHub Studio sigue abierto.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        {saved ? (
          <>
            <DialogHeader>
              <DialogTitle>Combinación guardada</DialogTitle>
              <DialogDescription>
                {saved.title} ya está en la Libreta con {roundsText(saved.examples.length)} de ejemplo.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                Seguir aquí
              </Button>
              <Button asChild>
                <Link href={playHref(saved.id)}>Abrir la jugada</Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardar la combinación</DialogTitle>
              <DialogDescription>
                Se guarda como jugada con la combinación en las notas y {roundsText(kept)} de ejemplo
                {kept < view.round_count ? `, las primeras de ${view.round_count}` : ''}.
              </DialogDescription>
            </DialogHeader>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Título
              <Input value={title} maxLength={120} onChange={(event) => setTitle(event.target.value)} />
            </label>
            {error ? (
              <p role="alert" className="text-body-sm text-destructive">
                {error}
              </p>
            ) : null}
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
                Cancelar
              </Button>
              <Button type="submit" disabled={busy || title.trim() === ''}>
                Guardar jugada
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** Builds the label of any key from the full playbook's tactics. */
export function comboLabeler(playbook: TeamPlaybook): LabelOf {
  const byKey = new Map(playbook.tactics.map((tactic) => [tactic.key, tacticLabel(tactic)]));
  return (key) => byKey.get(key) ?? key;
}

