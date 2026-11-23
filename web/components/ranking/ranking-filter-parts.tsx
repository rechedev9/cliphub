'use client';

import { useEffect, useId, useState, type ReactNode } from 'react';
import { Pin, Search, X } from 'lucide-react';
import type { LeaderboardRow } from '@/lib/api/baseline';
import { MAX_MIN_ROUNDS, searchPlayers, type PinnedPlayer } from '@/lib/ranking/ranking';
import { Input } from '@/components/ui/input';
import { Toggle } from '@/components/ui/toggle';
import { cn } from '@/lib/utils';

/** The slider covers the useful range; the number box takes anything up to the cap. */
const SLIDER_MAX = 500;

const TONE = {
  neutral: '',
  ct: 'border-side-ct/40 bg-side-ct/10 text-side-ct hover:border-side-ct hover:bg-side-ct/20 hover:text-side-ct data-[state=on]:border-side-ct data-[state=on]:bg-side-ct/25 data-[state=on]:text-side-ct',
  t: 'border-side-t/40 bg-side-t/10 text-side-t hover:border-side-t hover:bg-side-t/20 hover:text-side-t data-[state=on]:border-side-t data-[state=on]:bg-side-t/25 data-[state=on]:text-side-t',
} as const;

/** One labelled row of the filter card: a tiny tracked caps label at the left, the controls after it. */
export function FilterRow({ label, children }: { label: string; children: ReactNode }): ReactNode {
  return (
    <div className="flex flex-col gap-1.5 sm:flex-row sm:items-start sm:gap-3">
      <span className="shrink-0 text-meta sm:w-18 sm:pt-1.5 font-semibold tracking-widest text-fg-3 uppercase">{label}</span>
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">{children}</div>
    </div>
  );
}

/** A pill of a filter row: neutral for maps and library roles, blue for CT, orange for T. */
export function FilterChip({
  pressed,
  tone = 'neutral',
  title,
  disabled,
  onPick,
  children,
}: {
  pressed: boolean;
  tone?: keyof typeof TONE;
  title?: string;
  disabled?: boolean;
  onPick: () => void;
  children: ReactNode;
}): ReactNode {
  return (
    <Toggle
      variant="filter"
      size="sm"
      pressed={pressed}
      title={title}
      disabled={disabled}
      onPressedChange={onPick}
      className={cn('h-6.5 min-w-0 px-2.5 font-semibold', TONE[tone])}
    >
      {children}
    </Toggle>
  );
}

/** A whole number committed on release, blur or Enter, so a drag or typing "120" does not ask for every step. */
export function MinRoundsControl({ value, fallback, onCommit }: { value: number; fallback: number; onCommit: (rounds: number) => void }): ReactNode {
  const [text, setText] = useState(String(value));
  const hintId = useId();
  useEffect(() => setText(String(value)), [value]);
  function commit(next: number): void {
    if (Number.isInteger(next) && next >= 1 && next <= MAX_MIN_ROUNDS) {
      if (next !== value) onCommit(next);
    } else {
      setText(String(value));
    }
  }
  const parsed = Number(text);
  const slider = Number.isFinite(parsed) ? Math.min(Math.max(parsed, 1), SLIDER_MAX) : value;
  return (
    <div className="flex w-full flex-wrap items-center gap-x-2.5 gap-y-1 sm:ml-auto sm:w-auto">
      <span className="text-meta font-semibold tracking-widest text-fg-3 uppercase">Rondas mín.</span>
      <input
        type="range"
        min={1}
        max={SLIDER_MAX}
        value={slider}
        aria-label="Rondas mínimas, deslizador"
        onChange={(event) => setText(event.target.value)}
        onPointerUp={(event) => commit(Number(event.currentTarget.value))}
        onKeyUp={(event) => commit(Number(event.currentTarget.value))}
        className="h-1.5 min-w-24 flex-1 cursor-pointer accent-primary sm:w-40 sm:flex-none"
      />
      <Input
        type="number"
        inputMode="numeric"
        min={1}
        max={MAX_MIN_ROUNDS}
        value={text}
        aria-label="Rondas mínimas"
        aria-describedby={hintId}
        onChange={(event) => setText(event.target.value)}
        onBlur={() => commit(Number(text))}
        onKeyDown={(event) => {
          if (event.key === 'Enter') commit(Number(text));
        }}
        className="h-7 w-16 px-2 text-center text-body-sm font-semibold tabular-nums"
      />
      <p id={hintId} className="basis-full text-meta text-fg-3 sm:text-right">
        Por defecto, {fallback} rondas: con menos, una sola buena partida basta para subir a lo más alto.
      </p>
    </div>
  );
}

/** Search among the ranked players and pin one, so a player can be followed through any filter. */
export function PlayerSearch({
  players,
  pins,
  onPin,
}: {
  players: readonly LeaderboardRow[];
  pins: readonly PinnedPlayer[];
  onPin: (pin: PinnedPlayer) => void;
}): ReactNode {
  const [term, setTerm] = useState('');
  const found = searchPlayers({ rows: players, term, pins });
  return (
    <div className="relative">
      <Search aria-hidden className="pointer-events-none absolute top-2 left-2 size-3.5 text-fg-3" />
      <Input
        type="search"
        value={term}
        placeholder="Buscar jugadores…"
        aria-label="Buscar jugadores para fijar"
        onChange={(event) => setTerm(event.target.value)}
        className="h-7 w-48 pl-7 text-body-sm"
      />
      {found.length > 0 ? (
        <ul className="studio-panel absolute z-10 mt-1 w-64 p-1" aria-label="Jugadores encontrados">
          {found.map((row) => (
            <li key={row.steamid64}>
              <button
                type="button"
                onClick={() => {
                  onPin({ steamid64: row.steamid64, name: row.name });
                  setTerm('');
                }}
                className="flex h-8 w-full items-center gap-2 rounded-sm px-2 text-left text-body-sm text-fg-1 outline-none hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50"
              >
                <Pin aria-hidden className="size-3 shrink-0 text-fg-3" />
                <span className="truncate">{row.name}</span>
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** The pinned players as chips; a click unpins, in step with the MCP's pinned list. */
export function PinnedChips({ pins, onPin }: { pins: readonly PinnedPlayer[]; onPin: (pin: PinnedPlayer) => void }): ReactNode {
  return (
    <>
      {pins.map((pin) => (
        <button
          key={pin.steamid64}
          type="button"
          aria-label={`Dejar de fijar a ${pin.name}`}
          title="Dejar de fijar"
          onClick={() => onPin(pin)}
          className="inline-flex h-6.5 max-w-40 items-center gap-1.5 rounded-md border border-border-strong bg-surface-2 px-2.5 text-meta font-semibold text-fg-1 outline-none hover:border-fg-4 hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          <span className="truncate">{pin.name}</span>
          <X aria-hidden className="size-3 shrink-0 text-fg-3" />
        </button>
      ))}
    </>
  );
}
