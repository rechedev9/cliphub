'use client';

import type { ReactNode } from 'react';
import {
  ADVANTAGE_MAX,
  HALF_BUY,
  OPPONENT_TOPS,
  SCOPE_ROUND_MAX,
  type AnalysisScope,
  type ScopeBuy,
  type ScopePresence,
  type ScopeRounds,
  type ScopeWhose,
} from '@/lib/analysis/scope';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

/** Radix Select reserves the empty string, so "every value" travels as this. */
const ALL = '__all__';
const CUSTOM = '__custom__';
const TRIGGER = 'h-8 gap-1.5 px-2.5 text-body-sm';

type Patch = (patch: Partial<AnalysisScope>) => void;
type Choice = { readonly value: string; readonly label: string; readonly disabled?: boolean };

function ChoiceSelect({
  label,
  allLabel,
  value,
  choices,
  onChange,
}: {
  label: string;
  allLabel: string;
  value: string;
  choices: readonly Choice[];
  onChange: (value: string) => void;
}): ReactNode {
  return (
    <Select value={value === '' ? ALL : value} onValueChange={(next) => onChange(next === ALL ? '' : next)}>
      <SelectTrigger aria-label={label} className={cn(TRIGGER, value !== '' && 'border-fg-3 text-fg-1')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>{allLabel}</SelectItem>
        {choices.map((choice) => (
          <SelectItem key={choice.value} value={choice.value} disabled={choice.disabled}>
            {choice.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

type Matchup = { readonly value: string; readonly label: string; readonly buy: ScopeBuy; readonly opponentBuy: ScopeBuy };

/** The economy matchups a person asks for most, each setting both buys at once. */
const MATCHUPS: readonly Matchup[] = [
  { value: 'full', label: 'Full contra full', buy: 'full', opponentBuy: 'full' },
  { value: 'pistol', label: 'Pistola contra pistola', buy: 'pistol', opponentBuy: 'pistol' },
  { value: 'half', label: 'Medias compras', buy: HALF_BUY, opponentBuy: '' },
  { value: 'eco', label: 'Ecos', buy: 'eco', opponentBuy: '' },
];

export function BuyMatchupSelect({ scope, onChange }: { scope: AnalysisScope; onChange: Patch }): ReactNode {
  const current = MATCHUPS.find((entry) => entry.buy === scope.buy && entry.opponentBuy === scope.opponentBuy);
  const custom = current === undefined && (scope.buy !== '' || scope.opponentBuy !== '');
  const value = current?.value ?? (custom ? CUSTOM : ALL);
  return (
    <Select
      value={value}
      onValueChange={(next) => {
        const matchup = MATCHUPS.find((entry) => entry.value === next);
        if (matchup !== undefined) onChange({ buy: matchup.buy, opponentBuy: matchup.opponentBuy });
        else if (next === ALL) onChange({ buy: '', opponentBuy: '' });
      }}
    >
      <SelectTrigger aria-label="Duelo de compras" className={cn(TRIGGER, value !== ALL && 'border-fg-3 text-fg-1')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL}>Cualquier duelo de compras</SelectItem>
        {MATCHUPS.map((entry) => (
          <SelectItem key={entry.value} value={entry.value}>
            {entry.label}
          </SelectItem>
        ))}
        {custom ? (
          <SelectItem value={CUSTOM} disabled>
            Duelo a medida
          </SelectItem>
        ) : null}
      </SelectContent>
    </Select>
  );
}

function parsePresence(value: string): ScopePresence {
  return value === 'with' || value === 'without' ? value : '';
}

export function AwpSelect({ rival, scope, onChange }: { rival: boolean; scope: AnalysisScope; onChange: Patch }): ReactNode {
  const whose = rival ? 'rival' : 'propia';
  return (
    <ChoiceSelect
      label={`AWP ${whose}`}
      allLabel={`AWP ${whose}: da igual`}
      value={rival ? scope.opponentAwp : scope.awp}
      choices={[
        { value: 'with', label: `Con AWP ${whose}` },
        { value: 'without', label: `Sin AWP ${whose}` },
      ]}
      onChange={(next) => onChange(rival ? { opponentAwp: parsePresence(next) } : { awp: parsePresence(next) })}
    />
  );
}

function advantageChoices(): Choice[] {
  const out: Choice[] = [];
  for (let own = ADVANTAGE_MAX; own >= 1; own -= 1) {
    for (let rival = ADVANTAGE_MAX; rival >= 1; rival -= 1) {
      if (own === ADVANTAGE_MAX && rival === ADVANTAGE_MAX) continue;
      out.push({ value: `${own}v${rival}`, label: `${own} contra ${rival}` });
    }
  }
  return out;
}

const ADVANTAGES = advantageChoices();

export function AdvantageSelect({ scope, onChange }: { scope: AnalysisScope; onChange: Patch }): ReactNode {
  return (
    <ChoiceSelect
      label="Jugadores vivos, propios contra rivales"
      allLabel="Vivos: cualquier número"
      value={scope.advantage}
      choices={ADVANTAGES}
      onChange={(advantage) => onChange({ advantage })}
    />
  );
}

function roundEnd(text: string): number | null {
  const value = Number(text);
  return text !== '' && Number.isInteger(value) && value >= 1 && value <= SCOPE_ROUND_MAX ? value : null;
}

export function RoundRangeControl({ scope, onChange }: { scope: AnalysisScope; onChange: Patch }): ReactNode {
  const { from, to } = scope.rounds;
  const set = (rounds: ScopeRounds): void => onChange({ rounds });
  return (
    <span className="flex items-center gap-1 text-body-sm text-fg-2">
      Rondas
      <Input
        type="number"
        inputMode="numeric"
        aria-label="Desde la ronda"
        placeholder="1"
        min={1}
        max={SCOPE_ROUND_MAX}
        value={from ?? ''}
        onChange={(event) => set({ from: roundEnd(event.target.value), to })}
        className="h-8 w-16 px-2"
      />
      a
      <Input
        type="number"
        inputMode="numeric"
        aria-label="Hasta la ronda"
        placeholder="30"
        min={1}
        max={SCOPE_ROUND_MAX}
        value={to ?? ''}
        onChange={(event) => set({ from, to: roundEnd(event.target.value) })}
        className="h-8 w-16 px-2"
      />
    </span>
  );
}

export function OpponentTopSelect({ scope, onChange }: { scope: AnalysisScope; onChange: Patch }): ReactNode {
  return (
    <ChoiceSelect
      label="Nivel del rival"
      allLabel="Nivel del rival: todos"
      value={scope.opponentTop === null ? '' : String(scope.opponentTop)}
      choices={OPPONENT_TOPS.map((top) => ({ value: String(top), label: `Rival en el top ${top} HLTV` }))}
      onChange={(next) => onChange({ opponentTop: OPPONENT_TOPS.find((top) => String(top) === next) ?? null })}
    />
  );
}

function parseWhose(value: string): ScopeWhose {
  return value === 'own' || value === 'others' ? value : '';
}

/** Needs a team marked as Mi equipo; until then both choices stay off. */
export function WhoseSelect({ scope, ownTeams, onChange }: { scope: AnalysisScope; ownTeams: number; onChange: Patch }): ReactNode {
  const off = ownTeams === 0;
  return (
    <ChoiceSelect
      label="De quién son las partidas"
      allLabel="Partidas de todos"
      value={scope.whose}
      choices={[
        { value: 'own', label: off ? 'Partidas de mi equipo (marca tu equipo al editarlo)' : 'Partidas de mi equipo', disabled: off },
        { value: 'others', label: 'Partidas de otros equipos', disabled: off },
      ]}
      onChange={(next) => onChange({ whose: parseWhose(next) })}
    />
  );
}
