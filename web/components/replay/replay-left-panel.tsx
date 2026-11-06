'use client';

import { useState, type ReactNode } from 'react';
import { Bomb, ChevronRight, Footprints, Flame, Tag } from 'lucide-react';
import type { CentreRound } from '@/lib/replay/centre-round';
import { LAYER_LABELS } from '@/lib/replay/layers';
import type { LayerToggle, ReplayLayers } from '@/lib/replay/layers';
import { ROUND_OUTCOME_LABEL, roundOutcome } from '@/lib/replay/round-outcome';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { ReplayChip, ReplayGroup } from './replay-chip';

const LAYER_ICONS: Record<LayerToggle, ReactNode> = {
  utility: <Flame aria-hidden />,
  bomb: <Bomb aria-hidden />,
  trails: <Footprints aria-hidden />,
  names: <Tag aria-hidden />,
};

const LAYER_ORDER: readonly LayerToggle[] = ['names', 'utility', 'bomb', 'trails'];

/** A section that folds; the count says what is inside while it is closed. */
function Fold({ title, count, defaultOpen, children }: { title: string; count?: number; defaultOpen: boolean; children: ReactNode }): ReactNode {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className="flex flex-col gap-1.5" aria-label={title}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className={cn('flex h-7 items-center gap-1.5 rounded-sm text-meta font-medium text-fg-3 outline-none hover:text-fg-1', FOCUS_RING)}
      >
        <ChevronRight className={cn('size-3.5 transition-transform duration-(--dur-fast)', open && 'rotate-90')} aria-hidden />
        {title}
        {count === undefined || count === 0 ? null : <span className="tabular-nums text-fg-2">{count}</span>}
      </button>
      {open ? children : null}
    </section>
  );
}

/** What this round was: both buys, both shapes, the site and how it ended. */
function RoundContext({ round }: { round: CentreRound }): ReactNode {
  const { economy, class: shape, end_reason: endReason } = round.round;
  const rows: [string, ReactNode][] = [
    ['Compra', <>
      <span className="text-side-ct">{buyLabel(economy.ct_buy)}</span>
      <span className="text-fg-3"> vs </span>
      <span className="text-side-t">{buyLabel(economy.t_buy)}</span>
    </>],
    ['Forma', <>
      <span className="text-side-t">{tPatternLabel(shape.t_side)}</span>
      <span className="text-fg-3"> · </span>
      <span className="text-side-ct">{ctPatternLabel(shape.ct_side)}</span>
    </>],
    ['Sitio', siteLabel(shape.site)],
    ['Final', ROUND_OUTCOME_LABEL[roundOutcome(endReason)]],
  ];
  return (
    <dl className="grid grid-cols-[3.75rem_minmax(0,1fr)] gap-x-2 gap-y-1 text-body-sm">
      {rows.map(([label, value]) => (
        <div key={label} className="contents">
          <dt className="text-meta leading-5 text-fg-3">{label}</dt>
          <dd className="truncate text-fg-1">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * The left column: the round in a few facts and what to make of it, the filters and round types the
 * page passes in, the map layers as small toggles, and the saved moments.
 */
export function ReplayLeftPanel({
  focused,
  actions,
  filters,
  layers,
  onToggleLayer,
  onShowAll,
  bookmarks,
}: {
  focused: CentreRound | null;
  /** What can be made from this round: a saved moment, a video, a play. */
  actions: ReactNode;
  filters: ReactNode;
  layers: ReplayLayers;
  onToggleLayer: (key: LayerToggle) => void;
  onShowAll: () => void;
  bookmarks: { count: number; panel: ReactNode } | null;
}): ReactNode {
  const reasons = focused?.round.class.reasons ?? [];
  return (
    <div className="flex flex-col gap-4">
      {focused === null ? null : <RoundContext round={focused} />}
      {actions}
      {filters === undefined || filters === null ? null : <div className="border-t border-border-subtle pt-3">{filters}</div>}
      <ReplayGroup
        title="En el mapa"
        className="border-t border-border-subtle pt-3"
        aside={
          layers.hiddenSlots.length === 0 ? null : (
            <Button type="button" size="xs" variant="ghost" onClick={onShowAll} className="h-6">
              Mostrar {layers.hiddenSlots.length === 1 ? 'el oculto' : `los ${layers.hiddenSlots.length} ocultos`}
            </Button>
          )
        }
      >
        <div className="flex flex-wrap gap-1" role="group" aria-label="Capas del mapa">
          {LAYER_ORDER.map((key) => (
            <ReplayChip key={key} pressed={layers[key]} onClick={() => onToggleLayer(key)}>
              {LAYER_ICONS[key]}
              {LAYER_LABELS[key]}
            </ReplayChip>
          ))}
        </div>
      </ReplayGroup>
      {bookmarks === null ? null : (
        <Fold title="Momentos guardados" count={bookmarks.count} defaultOpen={false}>
          {bookmarks.panel}
        </Fold>
      )}
      {reasons.length === 0 ? null : (
        <Fold title="Por qué se clasificó así" defaultOpen={false}>
          <ul className="flex flex-col gap-1 font-mono text-meta break-words text-fg-2">
            {reasons.map((reason) => (
              <li key={reason}>{reason}</li>
            ))}
          </ul>
        </Fold>
      )}
    </div>
  );
}
