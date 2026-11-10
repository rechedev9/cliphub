'use client';

import { useState, type ReactNode } from 'react';
import { ChevronRight, Minus, Plus } from 'lucide-react';
import type { PlaybookTactic, TeamPlaybook } from '@/lib/api/playbook';
import { AWP_START_SECONDS, comboMode, tacticKindHint, tacticKindTitle, tacticLabel, type ComboMode } from '@/lib/analysis/playbook-combo';
import { EDITOR_PANELS, panelConditions, panelRows, type EditorPanel } from '@/lib/analysis/tactic-editor';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

type Toggle = (key: string, mode: ComboMode) => void;

/** One condition: what it asks, how many of the kept rounds show it, and + to keep them or - to drop them. */
function ConditionRow({ tactic, mode, onToggle }: { tactic: PlaybookTactic; mode: ComboMode | null; onToggle: Toggle }): ReactNode {
  const label = tacticLabel(tactic);
  const win = tactic.rounds === 0 ? '' : `, gana el ${Math.round(tactic.win_rate.pct)} %`;
  return (
    <li className={cn('flex min-h-8 items-center gap-1.5 rounded-sm px-1.5', mode === 'with' && 'bg-primary/15', mode === 'without' && 'bg-destructive/10')}>
      <span className={cn('min-w-0 flex-1 truncate text-body-sm', mode === 'without' ? 'text-fg-3 line-through' : 'text-fg-1')} title={label}>
        {label}
      </span>
      <span className="font-num shrink-0 text-meta text-fg-3 tabular-nums" title={`${tactic.rounds} ${tactic.rounds === 1 ? 'ronda' : 'rondas'}${win}`}>
        {tactic.rounds}
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
    </li>
  );
}

function Panel({ panel, playbook, view, onToggle }: { panel: EditorPanel; playbook: TeamPlaybook; view: TeamPlaybook; onToggle: Toggle }): ReactNode {
  const rows = panelRows({ playbook, view, kinds: panel.kinds });
  const active = panelConditions({ combo: view.combo, playbook, kinds: panel.kinds });
  const [open, setOpen] = useState(true);
  const many = rows.length > 1;
  return (
    <section aria-label={panel.title} className="flex flex-col rounded-sm border border-border bg-surface-2">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className={cn('flex h-8 items-center gap-1.5 rounded-sm px-2 text-body-sm font-semibold text-fg-1 outline-none', FOCUS_RING)}
      >
        <ChevronRight aria-hidden className={cn('size-3.5 text-fg-3 transition-transform duration-(--dur-fast)', open && 'rotate-90')} />
        {panel.title}
        {active === 0 ? null : <span className="font-num rounded-sm bg-primary/16 px-1 text-meta font-semibold text-primary tabular-nums">{active}</span>}
      </button>
      {open ? (
        <div className="flex max-h-72 flex-col gap-2 overflow-y-auto border-t border-border-subtle p-1.5 [scrollbar-width:thin]">
          {rows.length === 0 ? <p className="px-1.5 text-meta text-fg-3">Ninguna ronda de la combinación.</p> : null}
          {rows.map(({ kind, tactics }) => {
            const hint = tacticKindHint(kind, AWP_START_SECONDS);
            return (
              <div key={kind} className="flex flex-col gap-0.5">
                {many ? <h4 className="px-1.5 text-meta font-medium tracking-wider text-fg-3 uppercase">{tacticKindTitle(kind)}</h4> : null}
                {hint === '' ? null : <p className="px-1.5 text-meta text-fg-3">{hint}</p>}
                <ul className="flex flex-col gap-0.5">
                  {tactics.map((tactic) => (
                    <ConditionRow key={tactic.key} tactic={tactic} mode={comboMode(view.combo, tactic.key)} onToggle={onToggle} />
                  ))}
                </ul>
              </div>
            );
          })}
        </div>
      ) : null}
    </section>
  );
}

/**
 * The conditions of a tactic as foldable panels under the filter stack: round type, buys, utility and
 * player conditions. Each row counts over the rounds the combination keeps, as the Playbook does.
 */
export function ConditionPanels({ playbook, view, onToggle }: { playbook: TeamPlaybook; view: TeamPlaybook; onToggle: Toggle }): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      {EDITOR_PANELS.map((panel) => (
        <Panel key={panel.id} panel={panel} playbook={playbook} view={view} onToggle={onToggle} />
      ))}
    </div>
  );
}
