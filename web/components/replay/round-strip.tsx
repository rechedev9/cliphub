'use client';

import { useEffect, useRef, type ReactNode } from 'react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalSide } from '@/lib/api/tactical';
import { entryKey, stripGroups } from '@/lib/replay/playlist';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** What the strip knows about a round once its match is loaded. */
export type StripRoundInfo = { score: string; winner: TacticalSide | '' };

function winnerBar(winner: TacticalSide | '' | undefined): string {
  if (winner === TACTICAL_SIDES.ct) return 'bg-side-ct';
  if (winner === TACTICAL_SIDES.t) return 'bg-side-t';
  return 'bg-fg-4';
}

/**
 * The rounds of the playlist in play order, grouped by match. In multi-round
 * mode each round also has a box that puts it on the map with the focused one.
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

  useEffect(() => {
    const current = listRef.current?.querySelector('[aria-current="true"]');
    if (current instanceof HTMLElement) current.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, [index]);

  return (
    <div ref={listRef} className="flex gap-4 overflow-x-auto overscroll-x-contain pb-2 [scrollbar-gutter:stable]" role="list" aria-label="Rondas de la lista">
      {stripGroups(entries).map((group) => (
        <div key={`${group.matchId}:${group.start}`} role="listitem" className="flex shrink-0 flex-col gap-1.5">
          <span className="max-w-full truncate text-meta text-fg-3">{groupLabel(group.matchId)}</span>
          <ol className="flex gap-1.5">
            {entries.slice(group.start, group.end).map((entry, offset) => {
              const at = group.start + offset;
              const key = entryKey(entry);
              const info = roundInfo(entry);
              const selected = at === index;
              const included = multi?.selected.includes(key) ?? false;
              const blocked = multi?.blocked(entry) ?? null;
              return (
                <li key={key} className="flex flex-col gap-1">
                  <button
                    type="button"
                    onClick={() => onSelect(at)}
                    aria-current={selected ? 'true' : undefined}
                    aria-label={`Ronda ${entry.round}${info ? `, marcador ${info.score}` : ''}`}
                    className={cn(
                      'relative flex h-14 w-14 flex-col items-center justify-center gap-0.5 overflow-hidden rounded-md border text-fg-1 outline-none transition-[background-color,border-color]',
                      FOCUS_RING,
                      selected ? 'border-primary bg-surface-4' : 'border-border-strong bg-surface-2 hover:bg-surface-3',
                    )}
                  >
                    <span className={cn('absolute inset-x-0 top-0 h-1', winnerBar(info?.winner))} aria-hidden />
                    <span className="text-body-sm font-semibold tabular-nums">R{entry.round}</span>
                    <span className="text-meta tabular-nums text-fg-3">{info?.score ?? '··'}</span>
                  </button>
                  {multi === null ? null : (
                    <label
                      title={blocked ?? undefined}
                      className="flex h-6 items-center justify-center"
                    >
                      <input
                        type="checkbox"
                        className="accent-primary"
                        checked={included}
                        disabled={blocked !== null || selected}
                        onChange={() => onToggleMulti(key)}
                        aria-label={`Superponer la ronda ${entry.round}`}
                      />
                    </label>
                  )}
                </li>
              );
            })}
          </ol>
        </div>
      ))}
    </div>
  );
}
