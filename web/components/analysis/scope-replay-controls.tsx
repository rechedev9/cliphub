'use client';

import { useState, type KeyboardEvent, type ReactNode } from 'react';
import { ArrowLeftRight } from 'lucide-react';
import { END_REASON_LABEL, SITE_LABEL, WEAPON_LABEL } from '@/lib/analysis/scope-chips';
import { swappedBuys, type AnalysisScope } from '@/lib/analysis/scope';
import {
  EQUIP_MAX,
  SCOPE_END_REASONS,
  SCOPE_SITES,
  SCOPE_WEAPONS,
  equipEnd,
  orderedEquip,
  type ScopeEquip,
  type ScopeWeapon,
} from '@/lib/analysis/scope-replay-filters';
import { tacticChoices, type TacticOption } from '@/lib/analysis/scope-tactics';
import { ChoiceSelect } from '@/components/analysis/scope-round-controls';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Toggle } from '@/components/ui/toggle';

type Patch = (patch: Partial<AnalysisScope>) => void;
type Props = { scope: AnalysisScope; onChange: Patch };

const TOGGLE = 'h-8 px-2.5 text-body-sm';

export function SiteSelect({ scope, onChange }: Props): ReactNode {
  return (
    <ChoiceSelect
      label="Sitio atacado"
      allLabel="Sitio: cualquiera"
      value={scope.site}
      choices={SCOPE_SITES.map((site) => ({ value: site, label: SITE_LABEL[site] }))}
      onChange={(next) => onChange({ site: SCOPE_SITES.find((site) => site === next) ?? '' })}
    />
  );
}

export function OutcomeSelect({ scope, onChange }: Props): ReactNode {
  return (
    <ChoiceSelect
      label="Resultado de la ronda"
      allLabel="Ganadas y perdidas"
      value={scope.outcome}
      choices={[
        { value: 'win', label: 'Ganadas' },
        { value: 'loss', label: 'Perdidas' },
      ]}
      onChange={(next) => onChange({ outcome: next === 'win' || next === 'loss' ? next : '' })}
    />
  );
}

export function PhaseSelect({ scope, onChange }: Props): ReactNode {
  return (
    <ChoiceSelect
      label="Prórroga"
      allLabel="Prórroga: da igual"
      value={scope.phase}
      choices={[
        { value: 'overtime', label: 'Solo prórroga' },
        { value: 'regulation', label: 'Sin prórroga' },
      ]}
      onChange={(next) => onChange({ phase: next === 'overtime' || next === 'regulation' ? next : '' })}
    />
  );
}

export function EndReasonSelect({ scope, onChange }: Props): ReactNode {
  return (
    <ChoiceSelect
      label="Motivo del final de la ronda"
      allLabel="Motivo: cualquiera"
      value={scope.endReason}
      choices={SCOPE_END_REASONS.map((reason) => ({ value: reason, label: END_REASON_LABEL[reason] }))}
      onChange={(next) => onChange({ endReason: SCOPE_END_REASONS.find((reason) => reason === next) ?? '' })}
    />
  );
}

function weaponOf(value: string): ScopeWeapon {
  return SCOPE_WEAPONS.find((weapon) => weapon === value) ?? '';
}

export function WeaponSelect({ rival, scope, onChange }: Props & { rival: boolean }): ReactNode {
  const whose = rival ? 'rival' : 'propia';
  return (
    <ChoiceSelect
      label={`Arma ${whose}`}
      allLabel={`Arma ${whose}: cualquiera`}
      value={rival ? scope.opponentWeapon : scope.weapon}
      choices={SCOPE_WEAPONS.map((weapon) => ({ value: weapon, label: `${WEAPON_LABEL[weapon]} ${whose}` }))}
      onChange={(next) => onChange(rival ? { opponentWeapon: weaponOf(next) } : { weapon: weaponOf(next) })}
    />
  );
}

export function FlagToggle({ label, pressed, onPressedChange }: { label: string; pressed: boolean; onPressedChange: (pressed: boolean) => void }): ReactNode {
  return (
    <Toggle variant="outline" size="sm" className={TOGGLE} pressed={pressed} onPressedChange={onPressedChange}>
      {label}
    </Toggle>
  );
}

type EquipEndProps = { label: string; placeholder: string; value: number | null; onCommit: (value: number | null) => void };

/** One end of the range: typing edits a draft, and blur or Enter applies it, so each keystroke is no new query. */
function EquipEndInput({ label, placeholder, value, onCommit }: EquipEndProps): ReactNode {
  const [draft, setDraft] = useState<string | null>(null);
  const commit = (): void => {
    if (draft === null) return;
    setDraft(null);
    if (equipEnd(draft) !== value) onCommit(equipEnd(draft));
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>): void => {
    if (event.key === 'Enter') commit();
  };
  return (
    <Input
      type="number"
      inputMode="numeric"
      aria-label={label}
      placeholder={placeholder}
      min={1}
      max={EQUIP_MAX}
      step={50}
      value={draft ?? value ?? ''}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      onKeyDown={onKeyDown}
      className="h-8 w-20 px-2"
    />
  );
}

/** The custom buy: equipment value per player, from and to, next to the fixed buy types. */
export function EquipRangeControl({ rival, scope, onChange }: Props & { rival: boolean }): ReactNode {
  const range = rival ? scope.opponentEquip : scope.equip;
  const whose = rival ? 'rival' : 'propio';
  const set = (next: ScopeEquip): void => onChange(rival ? { opponentEquip: orderedEquip(next) } : { equip: orderedEquip(next) });
  return (
    <span className="flex items-center gap-1 text-body-sm text-fg-2" title="Valor del equipo por jugador al acabar las compras">
      Equipo {whose} $
      <EquipEndInput label={`Equipo ${whose} por jugador desde`} placeholder="0" value={range.from} onCommit={(from) => set({ from, to: range.to })} />
      a
      <EquipEndInput label={`Equipo ${whose} por jugador hasta`} placeholder="sin tope" value={range.to} onCommit={(to) => set({ from: range.from, to })} />
    </span>
  );
}

export function BuySwapButton({ scope, onChange }: Props): ReactNode {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      aria-label="Intercambiar compra propia y rival"
      title="Intercambiar compra propia y rival"
      onClick={() => onChange(swappedBuys(scope))}
    >
      <ArrowLeftRight aria-hidden />
    </Button>
  );
}

/** The Libreta plays of the team on the scope's map and side; one keeps only the rounds it was read from. */
export function TacticSelect({ scope, tactics, onChange }: Props & { tactics: readonly TacticOption[] }): ReactNode {
  const shown = tacticChoices(tactics, scope);
  const picked = tactics.find((option) => option.value === scope.tactic) ?? { value: scope.tactic, label: 'Jugada de la Libreta' };
  const choices = scope.tactic === '' || shown.some((option) => option.value === scope.tactic) ? shown : [...shown, picked];
  return (
    <ChoiceSelect
      label="Táctica de la Libreta"
      allLabel={shown.length === 0 ? 'Táctica: sin jugadas en este mapa y lado' : 'Táctica: cualquiera'}
      value={scope.tactic}
      choices={choices}
      onChange={(tactic) => onChange({ tactic })}
    />
  );
}
