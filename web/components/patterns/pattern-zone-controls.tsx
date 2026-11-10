'use client';

import type { ReactNode } from 'react';
import type { MapCallout } from '@/lib/api/callouts';
import { PATTERN_MAX_CALLOUTS, PATTERN_MAX_MIN_PLAYERS, type PatternBlock } from '@/lib/api/patterns';
import type { AreaShape } from '@/lib/analysis/pattern-finder';
import { Segmented } from '@/components/ui/segmented';

const SHAPES: readonly { value: AreaShape; label: string }[] = [
  { value: 'box', label: 'Caja' },
  { value: 'polygon', label: 'Polígono' },
  { value: 'callouts', label: 'Callouts' },
];

const HINT: Record<AreaShape, string> = {
  box: 'Arrastra sobre el radar para dibujar la caja; con zoom, Mayús + arrastrar.',
  polygon: 'Haz clic en el radar para poner cada esquina y cierra en la primera o con doble clic. Esc o clic derecho cancela.',
  callouts: 'Marca los callouts del mapa que forman la zona.',
};

type Props = {
  block: PatternBlock;
  /** How the zone is drawn: the area's own shape, or the one picked before drawing. */
  shape: AreaShape;
  onShape: (shape: AreaShape) => void;
  callouts: readonly MapCallout[];
  onChange: (block: PatternBlock) => void;
};

function calloutName(callout: MapCallout): string {
  return callout.name ?? callout.token;
}

/** The zone of a block: how it is drawn, the callouts that form it and how many players it needs at once. */
export function ZoneControls({ block, shape, onShape, callouts, onChange }: Props): ReactNode {
  const picked = block.area?.callouts ?? [];
  const toggle = (token: string): void => {
    const next = picked.includes(token) ? picked.filter((entry) => entry !== token) : [...picked, token].slice(0, PATTERN_MAX_CALLOUTS);
    const out: PatternBlock = { ...block };
    if (next.length === 0) delete out.area;
    else out.area = { callouts: next };
    onChange(out);
  };
  const chosen = block.players?.length ?? 0;
  const most = chosen > 0 ? Math.min(chosen, PATTERN_MAX_MIN_PLAYERS) : PATTERN_MAX_MIN_PLAYERS;
  const setNeed = (value: string): void => {
    const out: PatternBlock = { ...block };
    const need = Number(value);
    if (need <= 1) delete out.min_players;
    else out.min_players = need;
    onChange(out);
  };
  const sorted = [...callouts].sort((a, b) => calloutName(a).localeCompare(calloutName(b), 'es'));

  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-meta text-fg-2">Zona</span>
        <Segmented<AreaShape>
          label="Cómo se marca la zona"
          value={shape}
          options={SHAPES.map((entry) => ({ ...entry, disabled: entry.value === 'callouts' && callouts.length === 0, reason: 'Este mapa no tiene callouts' }))}
          onChange={onShape}
        />
      </div>
      {block.area === undefined ? <p className="text-meta text-fg-3">{HINT[shape]}</p> : null}
      {shape === 'callouts' && callouts.length > 0 ? (
        <fieldset className="flex max-h-40 flex-col gap-1 overflow-y-auto rounded-sm border border-border bg-surface-0 px-2 py-1.5">
          <legend className="sr-only">Callouts de la zona</legend>
          {sorted.map((callout) => (
            <label key={callout.token} className="flex items-center gap-2 text-body-sm text-fg-1">
              <input
                type="checkbox"
                className="accent-primary"
                checked={picked.includes(callout.token)}
                disabled={!picked.includes(callout.token) && picked.length >= PATTERN_MAX_CALLOUTS}
                onChange={() => toggle(callout.token)}
              />
              <span className="truncate">{calloutName(callout)}</span>
            </label>
          ))}
        </fieldset>
      ) : null}
      {block.kind === 'player' && block.area !== undefined ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-meta text-fg-2">A la vez en la zona</span>
          <Segmented<string>
            label="Jugadores a la vez en la zona"
            value={String(Math.min(block.min_players ?? 1, most))}
            options={Array.from({ length: PATTERN_MAX_MIN_PLAYERS }, (_, i) => ({
              value: String(i + 1),
              label: i === 0 ? 'Uno' : `${i + 1} o más`,
              disabled: i + 1 > most,
              reason: 'No hay tantos jugadores marcados',
            }))}
            onChange={setNeed}
          />
        </div>
      ) : null}
    </div>
  );
}
