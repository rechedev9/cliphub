'use client';

import type { ReactNode } from 'react';
import { GRENADE_KIND_LABEL } from '@/lib/analysis/grenade-patterns';
import { grenadeIcon } from '@/lib/analysis/icons';
import { TEAM_GRENADE_KINDS, type TeamGrenadeKind } from '@/lib/api/grenades';
import {
  LINEUP_CLICKS,
  LINEUP_MOVEMENTS,
  isLineupClick,
  isLineupGrenade,
  isLineupMovement,
  type LibraryLineup,
} from '@/lib/api/lineups';
import { CLICK_LABEL, MOVEMENT_LABEL, techniqueLabel } from '@/lib/libreta/lineups';
import { Badge } from '@/components/ui/badge';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { Field } from './play-fields';

export type LineupTechnique = Pick<LibraryLineup, 'movement' | 'jump' | 'click'>;

/** The grenade's silhouette in its kind's colour, beside its name. */
export function GrenadeName({ kind }: { kind: TeamGrenadeKind }): ReactNode {
  return (
    <span className="inline-flex items-center gap-1.5">
      <span aria-hidden className="flex w-4 justify-center" style={{ color: `var(${GRENADE_KIND_TOKEN[kind]})` }}>
        <EquipmentIcon name={grenadeIcon(kind)} height={14} />
      </span>
      {GRENADE_KIND_LABEL[kind]}
    </span>
  );
}

/** The technique, marked when Studio read it from the demo and nobody has checked it yet. */
export function TechniqueText({ lineup }: { lineup: LineupTechnique & Pick<LibraryLineup, 'technique_estimated'> }): ReactNode {
  return (
    <span className="inline-flex items-center gap-1.5">
      {techniqueLabel(lineup)}
      {lineup.technique_estimated ? (
        <Badge variant="outline" title="Studio la leyó de la demo. Compruébala en CS2 y corrígela si hace falta.">
          estimada
        </Badge>
      ) : null}
    </span>
  );
}

export function GrenadeField({ value, onChange }: { value: TeamGrenadeKind; onChange: (kind: TeamGrenadeKind) => void }): ReactNode {
  return (
    <Field label="Granada">
      <Select value={value} onValueChange={(next) => isLineupGrenade(next) && onChange(next)}>
        <SelectTrigger aria-label="Granada" className="h-9 w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {TEAM_GRENADE_KINDS.map((kind) => (
            <SelectItem key={kind} value={kind}>
              {GRENADE_KIND_LABEL[kind]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}

/** How the grenade is thrown: movement, jump and which click. */
export function TechniqueFields({ value, onChange }: { value: LineupTechnique; onChange: (next: LineupTechnique) => void }): ReactNode {
  return (
    <div className="flex flex-col gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Movimiento">
          <Select value={value.movement} onValueChange={(next) => isLineupMovement(next) && onChange({ ...value, movement: next })}>
            <SelectTrigger aria-label="Movimiento" className="h-9 w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LINEUP_MOVEMENTS.map((movement) => (
                <SelectItem key={movement} value={movement}>
                  {MOVEMENT_LABEL[movement]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label="Clic">
          <Select value={value.click} onValueChange={(next) => isLineupClick(next) && onChange({ ...value, click: next })}>
            <SelectTrigger aria-label="Clic" className="h-9 w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {LINEUP_CLICKS.map((click) => (
                <SelectItem key={click} value={click}>
                  {CLICK_LABEL[click]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
      </div>
      <label className="flex items-center gap-2 text-body-sm text-fg-1">
        <input
          type="checkbox"
          checked={value.jump}
          onChange={(event) => onChange({ ...value, jump: event.target.checked })}
          className="size-4 accent-primary"
        />
        Con salto
      </label>
    </div>
  );
}
