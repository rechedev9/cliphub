'use client';

import type { ReactNode } from 'react';
import { Plus, RotateCcw, X } from 'lucide-react';
import type { LibraryRosterEntry } from '@/lib/api/library';
import { PATTERN_MAX_BLOCKS, PATTERN_MAX_PLAYERS, type PatternBlock, type PatternBlockKind, type PatternDefinition, type PatternResult } from '@/lib/api/patterns';
import { TEAM_GRENADE_KINDS, type TeamGrenadeKind } from '@/lib/api/grenades';
import { GRENADE_KIND_LABEL } from '@/lib/analysis/grenade-patterns';
import { BLOCK_LABEL, INVERT_LABEL, RIVAL_LABEL, blockSentence, newBlock, patternPlayerToken } from '@/lib/analysis/pattern-finder';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';

const ADD_ORDER: readonly PatternBlockKind[] = ['player', 'utility', 'plant'];

type Props = {
  pattern: PatternDefinition;
  active: number;
  onActive: (index: number) => void;
  onChange: (pattern: PatternDefinition) => void;
  onReset: () => void;
  result: PatternResult | null;
  /** The team's players, most recent first. */
  players: readonly LibraryRosterEntry[];
};

/** The blocks of a pattern, each a condition every kept round meets, and the buttons that add more. */
export function PatternBlocks({ pattern, active, onActive, onChange, onReset, result, players }: Props): ReactNode {
  const setBlock = (index: number, block: PatternBlock): void => onChange({ ...pattern, blocks: pattern.blocks.map((entry, i) => (i === index ? block : entry)) });
  const remove = (index: number): void => {
    onChange({ ...pattern, blocks: pattern.blocks.filter((_, i) => i !== index) });
    onActive(Math.max(0, Math.min(active, pattern.blocks.length - 2)));
  };
  const add = (kind: PatternBlockKind): void => {
    onChange({ ...pattern, blocks: [...pattern.blocks, newBlock(kind)] });
    onActive(pattern.blocks.length);
  };
  const full = pattern.blocks.length >= PATTERN_MAX_BLOCKS;

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-body font-semibold text-fg-1">Qué buscar</h2>
        <Button type="button" variant="ghost" size="xs" onClick={onReset}>
          <RotateCcw aria-hidden />
          Empezar de cero
        </Button>
      </div>
      {pattern.blocks.map((block, index) => (
        <BlockCard
          key={index}
          block={block}
          active={index === active}
          count={result?.blocks[index] ?? null}
          players={players}
          removable={pattern.blocks.length > 1}
          onSelect={() => onActive(index)}
          onChange={(next) => setBlock(index, next)}
          onRemove={() => remove(index)}
        />
      ))}
      <div className="flex flex-wrap gap-1.5">
        {ADD_ORDER.map((kind) => (
          <Button key={kind} type="button" variant="outline" size="xs" disabled={full} onClick={() => add(kind)}>
            <Plus aria-hidden />
            {BLOCK_LABEL[kind]}
          </Button>
        ))}
      </div>
    </div>
  );
}

function BlockCard({
  block,
  active,
  count,
  players,
  removable,
  onSelect,
  onChange,
  onRemove,
}: {
  block: PatternBlock;
  active: boolean;
  count: { matched: number; rounds: number } | null;
  players: readonly LibraryRosterEntry[];
  removable: boolean;
  onSelect: () => void;
  onChange: (block: PatternBlock) => void;
  onRemove: () => void;
}): ReactNode {
  const chosen = block.players ?? [];
  const togglePlayer = (id: string): void => {
    const next = chosen.includes(id) ? chosen.filter((entry) => entry !== id) : [...chosen, id].slice(0, PATTERN_MAX_PLAYERS);
    const out: PatternBlock = { ...block, players: next };
    if (next.length === 0) delete out.players;
    onChange(out);
  };
  const flag = (key: 'rival' | 'invert' | 'from_throw', on: boolean): void => {
    const out: PatternBlock = { ...block };
    if (on) out[key] = true;
    else delete out[key];
    if (key === 'rival') delete out.players;
    onChange(out);
  };
  const setGrenade = (value: TeamGrenadeKind | 'any'): void => {
    const out: PatternBlock = { ...block };
    if (value === 'any') delete out.grenade;
    else out.grenade = value;
    onChange(out);
  };
  const clearArea = (): void => {
    const out: PatternBlock = { ...block };
    delete out.area;
    onChange(out);
  };

  return (
    <section
      aria-label={BLOCK_LABEL[block.kind]}
      onFocusCapture={onSelect}
      onPointerDown={onSelect}
      className={cn('flex flex-col gap-2 rounded-md border bg-surface-1 px-3 py-2.5', active ? 'border-primary' : 'border-border')}
    >
      <header className="flex items-center justify-between gap-2">
        <h3 className="text-body-sm font-semibold text-fg-1">{BLOCK_LABEL[block.kind]}</h3>
        <span className="ml-auto text-meta tabular-nums text-fg-2">{count === null ? '' : `${count.matched} de ${count.rounds} rondas`}</span>
        {removable ? (
          <Button type="button" variant="ghost" size="icon-xs" aria-label={`Quitar el bloque ${BLOCK_LABEL[block.kind]}`} onClick={onRemove}>
            <X aria-hidden />
          </Button>
        ) : null}
      </header>
      {block.kind === 'player' && block.rival !== true && players.length > 0 ? (
        <fieldset className="flex flex-col gap-1">
          <legend className="sr-only">Jugadores</legend>
          {players.map((player) => {
            const index = chosen.indexOf(player.steamid64);
            return (
              <label key={player.steamid64} className="flex items-center gap-2 text-body-sm text-fg-1">
                <input type="checkbox" className="accent-primary" checked={index >= 0} onChange={() => togglePlayer(player.steamid64)} />
                <span
                  aria-hidden
                  className="size-2.5 shrink-0 rounded-full"
                  style={{ background: index >= 0 ? `var(${patternPlayerToken(index)})` : 'var(--border-strong)' }}
                />
                <span className="truncate">{player.nick || player.steamid64}</span>
              </label>
            );
          })}
          {chosen.length === 0 ? <p className="text-meta text-fg-3">Sin marcar, vale cualquiera del equipo.</p> : null}
        </fieldset>
      ) : null}
      {block.kind === 'utility' ? (
        <div className="flex flex-col gap-1.5">
          <Segmented<TeamGrenadeKind | 'any'>
            label="Granada"
            value={block.grenade ?? 'any'}
            options={[{ value: 'any', label: 'Cualquiera' }, ...TEAM_GRENADE_KINDS.map((kind) => ({ value: kind, label: GRENADE_KIND_LABEL[kind] }))]}
            onChange={setGrenade}
          />
          <Segmented<'landing' | 'throw'>
            label="La zona marca"
            value={block.from_throw === true ? 'throw' : 'landing'}
            options={[
              { value: 'landing', label: 'Dónde cae' },
              { value: 'throw', label: 'Desde dónde se tira' },
            ]}
            onChange={(value) => flag('from_throw', value === 'throw')}
          />
        </div>
      ) : null}
      <p className="text-body-sm text-fg-2">{blockSentence(block)}</p>
      {block.area === undefined && active ? <p className="text-meta text-fg-3">Arrastra sobre el radar para dibujar una zona; con zoom, Mayús + arrastrar.</p> : null}
      {block.area === undefined ? null : (
        <div className="flex flex-wrap gap-1.5">
          <Button type="button" variant="ghost" size="xs" onClick={clearArea}>
            <X aria-hidden />
            Quitar zona
          </Button>
        </div>
      )}
      <label className="flex items-center gap-2 text-body-sm text-fg-1">
        <input type="checkbox" className="accent-primary" checked={block.rival === true} onChange={(event) => flag('rival', event.currentTarget.checked)} />
        {RIVAL_LABEL}
      </label>
      <label className="flex items-center gap-2 text-body-sm text-fg-1">
        <input type="checkbox" className="accent-primary" checked={block.invert === true} onChange={(event) => flag('invert', event.currentTarget.checked)} />
        {INVERT_LABEL[block.kind]}
      </label>
    </section>
  );
}
