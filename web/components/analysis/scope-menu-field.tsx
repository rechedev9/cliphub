'use client';

import type { ReactNode } from 'react';
import type { AnalysisScope, ScopeField } from '@/lib/analysis/scope';
import type { TacticOption } from '@/lib/analysis/scope-tactics';
import type { ScopeOption } from '@/components/analysis/scope-bar';
import {
  AdvantageSelect,
  AwpSelect,
  ChoiceSelect,
  OpponentTopSelect,
  RoundRangeControl,
  WhoseSelect,
} from '@/components/analysis/scope-round-controls';
import {
  EndReasonSelect,
  FlagToggle,
  OutcomeSelect,
  PhaseSelect,
  SiteSelect,
  TacticSelect,
  WeaponSelect,
} from '@/components/analysis/scope-replay-controls';
import { EventChoices, ExcludeTacticChoices, MatchTagChoices } from '@/components/analysis/scope-text-controls';

export type MenuFieldProps = {
  field: ScopeField;
  scope: AnalysisScope;
  onChange: (patch: Partial<AnalysisScope>) => void;
  opponents: readonly ScopeOption[];
  tactics: readonly TacticOption[];
  ownTeams: number;
};

/** The control of one field that lives in the "Filtros" popover. */
export function MenuField({ field, scope, onChange, opponents, tactics, ownTeams }: MenuFieldProps): ReactNode {
  switch (field) {
    case 'opponent':
      return (
        <ChoiceSelect label="Rival" allLabel="Todos los rivales" value={scope.opponent} choices={opponents} onChange={(opponent) => onChange({ opponent })} />
      );
    case 'awp':
      return <AwpSelect rival={false} scope={scope} onChange={onChange} />;
    case 'opponentAwp':
      return <AwpSelect rival scope={scope} onChange={onChange} />;
    case 'advantage':
      return <AdvantageSelect scope={scope} onChange={onChange} />;
    case 'rounds':
      return <RoundRangeControl scope={scope} onChange={onChange} />;
    case 'opponentTop':
      return <OpponentTopSelect scope={scope} onChange={onChange} />;
    case 'whose':
      return <WhoseSelect scope={scope} ownTeams={ownTeams} onChange={onChange} />;
    case 'site':
      return <SiteSelect scope={scope} onChange={onChange} />;
    case 'outcome':
      return <OutcomeSelect scope={scope} onChange={onChange} />;
    case 'postPlant':
      return <FlagToggle label="Solo postplant" pressed={scope.postPlant} onPressedChange={(postPlant) => onChange({ postPlant })} />;
    case 'phase':
      return <PhaseSelect scope={scope} onChange={onChange} />;
    case 'secondRound':
      return <FlagToggle label="Segunda ronda" pressed={scope.secondRound} onPressedChange={(secondRound) => onChange({ secondRound })} />;
    case 'endReason':
      return <EndReasonSelect scope={scope} onChange={onChange} />;
    case 'weapon':
      return <WeaponSelect rival={false} scope={scope} onChange={onChange} />;
    case 'opponentWeapon':
      return <WeaponSelect rival scope={scope} onChange={onChange} />;
    case 'tactic':
      return <TacticSelect scope={scope} tactics={tactics} onChange={onChange} />;
    case 'excludeTactics':
      return <ExcludeTacticChoices scope={scope} tactics={tactics} onChange={onChange} />;
    case 'matchTags':
      return <MatchTagChoices scope={scope} onChange={onChange} exclude={false} />;
    case 'excludeMatchTags':
      return <MatchTagChoices scope={scope} onChange={onChange} exclude />;
    case 'events':
      return <EventChoices scope={scope} onChange={onChange} />;
    default:
      return null;
  }
}
