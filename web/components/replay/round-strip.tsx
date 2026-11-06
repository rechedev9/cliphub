'use client';

import { useEffect, useRef, type ReactNode } from 'react';
import { Bomb, Clock3, Scissors, Skull, type LucideIcon } from 'lucide-react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import { entryKey, stripGroups } from '@/lib/replay/playlist';
import { ROUND_OUTCOME_LABEL } from '@/lib/replay/round-outcome';
import type { RoundOutcome } from '@/lib/replay/round-outcome';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** What the strip knows about a round once its match is loaded. */
export type StripRoundInfo = { score: string; winner: TacticalSide | ''; outcome: RoundOutcome; half: number };

const OUTCOME_ICON: Record<RoundOutcome, LucideIcon | null> = {
  exploded: Bomb,
  defused: Scissors,
  elimination: Skull,
  time: Clock3,
  other: null,
};

function sideText(winner: TacticalSide | '' | undefined): string {
  if (winner === TACTICAL_SIDES.ct) return 'text-side-ct';
  if (winner === TACTICAL_SIDES.t) return 'text-side-t';
  return 'text-fg-4';
}

function sideBar(winner: TacticalSide | '' | undefined): string {
  if (winner === TACTICAL_SIDES.ct) return 'bg-side-ct';
  if (winner === TACTICAL_SIDES.t) return 'bg-side-t';
  return 'bg-transparent';
}

/**
 * The rounds of the playlist in play order, grouped by match: the number, how
 * it ended in the winner's colour, and a gap at the half. In multi-round mode a
 * second press of a round adds it to the map instead of opening it.
 */
export function RoundStrip({
  entries,
  index,
  onSelect,
  groupLabel,
  roundInfo,
  multi,
  onToggleMulti,
}: {
  entries: readonly ReplayRef[];
  index: number;
  onSelect: (index: number) => void;
  /** `Mirage · NAVI vs FaZe`, or a placeholder while the match loads. */
  groupLabel: (matchId: string) => string;
  roundInfo: (entry: ReplayRef) => StripRoundInfo | null;
  multi: { selected: readonly string[]; blocked: (entry: ReplayRef) => string | null } | null;
  onToggleMulti: (key: string) => void;
}): ReactNode {
  const listRef = useRef<HTMLDivElement>(null);
  const groups = stripGroups(entries);

  useEffect(() => {
    // Scrolls the strip only: scrollIntoView would also scroll the page under the replay.
    const list = listRef.current;
    const current = list?.querySelector('[aria-current="true"]');
    if (list === null || list === undefined || !(current instanceof HTMLElement)) return;
    const left = current.getBoundingClientRect().left - list.getBoundingClientRect().left + list.scrollLeft;
    if (left < list.scrollLeft) list.scrollLeft = left - 24;
    else if (left + current.offsetWidth > list.scrollLeft + list.clientWidth) list.scrollLeft = left + current.offsetWidth - list.clientWidth + 24;
  }, [index]);

  return (
    <div
      ref={listRef}
      className="flex min-w-0 flex-1 items-end gap-3 overflow-x-auto overscroll-x-contain [scrollbar-width:thin]"
      role="list"
      aria-label="Rondas de la lista"
      onWheel={(event) => {
        if (event.deltaY === 0 || event.currentTarget.scrollWidth <= event.currentTarget.clientWidth) return;
        event.currentTarget.scrollLeft += event.deltaY;
      }}
    >
      {groups.map((group) => (
        <div key={`${group.matchId}:${group.start}`} role="listitem" className="flex shrink-0 flex-col gap-1">
          {groups.length > 1 ? <span className="max-w-60 truncate text-meta text-fg-3">{groupLabel(group.matchId)}</span> : null}
          <ol className="flex">
            {entries.slice(group.start, group.end).map((entry, offset) => {
              const at = group.start + offset;
              const key = entryKey(entry);
              const info = roundInfo(entry);
              const previous = offset > 0 ? roundInfo(entries[at - 1]) : null;
              const halfBreak = info !== null && previous !== null && info.half !== previous.half;
              const selected = at === index;
              const included = multi?.selected.includes(key) ?? false;
              const blocked = multi?.blocked(entry) ?? null;
              const Icon = info === null ? null : OUTCOME_ICON[info.outcome];
              const outcome = info === null ? '' : `, ${ROUND_OUTCOME_LABEL[info.outcome].toLowerCase()}`;
              const toggles = multi !== null && !selected;
              let hint: string | undefined = info === null ? undefined : `${info.score}${outcome}`;
              if (toggles) hint = blocked ?? 'Pulsa para superponer, doble clic para abrir';
              return (
                <li key={key} className={cn('flex', halfBreak ? 'ml-3' : offset > 0 && 'ml-0.5')}>
                  <button
                    type="button"
                    onClick={() => (toggles ? onToggleMulti(key) : onSelect(at))}
                    onDoubleClick={() => onSelect(at)}
                    disabled={toggles && blocked !== null}
                    title={hint}
                    aria-current={selected ? 'true' : undefined}
                    aria-pressed={multi === null ? undefined : included || selected}
                    aria-label={`Ronda ${entry.round}${info ? `, marcador ${info.score}${outcome}` : ''}`}
                    className={cn(
                      'relative flex h-10 w-8 flex-col items-center justify-center gap-0.5 overflow-hidden rounded-sm text-fg-2 outline-none',
                      'transition-[background-color,color] duration-(--dur-fast) disabled:opacity-40',
                      FOCUS_RING,
                      selected ? 'bg-surface-5 text-fg-1 shadow-[inset_0_0_0_1px_var(--fg-2)]' : 'bg-surface-1 hover:bg-surface-3 hover:text-fg-1',
                      included && !selected && 'bg-surface-4 text-fg-1 shadow-[inset_0_0_0_1px_var(--border-accent)]',
                    )}
                  >
                    <span className="text-meta leading-none font-medium tabular-nums">{entry.round}</span>
                    {Icon === null ? <span className="size-3" aria-hidden /> : <Icon className={cn('size-3', sideText(info?.winner))} aria-hidden />}
                    <span className={cn('absolute inset-x-1 bottom-0 h-0.5 rounded-full', sideBar(info?.winner))} aria-hidden />
                  </button>
                </li>
              );
            })}
          </ol>
        </div>
      ))}
    </div>
  );
}
