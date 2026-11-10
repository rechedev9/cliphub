'use client';

import type { ReactNode } from 'react';
import type { Baseline } from '@/lib/api/baseline';
import type { SampleNotes, TeamLineups } from '@/lib/api/team-insights';
import type { ResultDot } from '@/lib/analysis/context-bar';
import type { MapCounts } from '@/lib/analysis/map-counts';
import type { AnalysisScope, ScopeField } from '@/lib/analysis/scope';
import type { TacticOption } from '@/lib/analysis/scope-tactics';
import { ScopeFilter } from '@/components/analysis/scope-filter';
import { PeriodField } from '@/components/analysis/scope-period';

/** `notes` says what the count leaves out or estimates: unrated rivals, the match cap, AWP from kills. */
export type ScopeCounts = { readonly maps: number; readonly rounds: number; readonly notes?: SampleNotes };

export type ScopeOption = { readonly value: string; readonly label: string };

/** One match of the sample, as a row of the "Mostrar partidas" list. */
export type ScopeMatch = {
  readonly id: string;
  readonly href: string;
  readonly label: string;
  readonly date: string;
  readonly outcome: ResultDot | null;
};

/** The team or player the page is about, switchable without leaving the tab. */
export type ScopeEntity = {
  /** What the picker chooses, such as "Equipo". */
  readonly noun: string;
  readonly id: string;
  readonly options: readonly ScopeOption[];
  /** Keep the last four opened in the picker's "Recientes" group. */
  readonly recents?: boolean;
  readonly hrefFor: (id: string) => string;
  /** Picks without navigating, for a page whose team is not in its path (the tools keep it in `equipo`). */
  readonly onSelect?: (id: string) => void;
  /** `team` paints the picker in the dark red of a team select. */
  readonly tone?: 'team';
};

export type ScopeBarProps = {
  scope: AnalysisScope;
  onChange: (patch: Partial<AnalysisScope>) => void;
  onReset: (fields: readonly ScopeField[]) => void;
  /** The controls this view reads, in this order; the others stay in the URL untouched. */
  fields: readonly ScopeField[];
  /** Map names for the map control, such as `de_mirage`. */
  maps?: readonly string[];
  /** The view reads one map at a time, so the empty choice asks for one instead of offering all. */
  mapRequired?: boolean;
  /** Matches per map in the scope, shown as "N partidas"; a map with 0 cannot be chosen. */
  mapCounts?: MapCounts | null;
  /** Rival teams for the opponent control. */
  opponents?: readonly ScopeOption[];
  /** The team's Libreta plays for the tactic control. */
  tactics?: readonly TacticOption[];
  /** The match limits offered, and the one the view uses while the URL names none. */
  limits?: { readonly options: readonly number[]; readonly fallback: number };
  /** Sample size of the answer on screen; null while it is being counted, absent to hide it. */
  counts?: ScopeCounts | null;
  /** Latest results in scope, oldest first, drawn as dots under the sample. */
  results?: readonly ResultDot[];
  entity?: ScopeEntity;
  /** The team's players for the lineup control; it shows only while the view reads `lineup`. */
  lineups?: TeamLineups | null;
  /** The pro reference the view reads, so the bar can say where Pro comes from; null while loading. */
  proReference?: Baseline | null;
  /** Teams marked as Mi equipo; with none, the whose-matches control stays off. */
  ownTeams?: number;
  /** Fixed context the person cannot change here, such as a report's rival and side. */
  context?: ReactNode;
  /** What the view's figures are compared with, such as the "Comparar con" teams; sits in the "Comparando con" widget. */
  comparison?: ReactNode;
  /** The matches behind the sample, listed under "Mostrar partidas"; absent hides the list. */
  matches?: readonly ScopeMatch[];
  className?: string;
};

/** The period control on its own, for screens that lay out their own filter card. */
export function PeriodControl({ scope, onChange }: { scope: AnalysisScope; onChange: ScopeBarProps['onChange'] }): ReactNode {
  return <PeriodField scope={scope} onChange={onChange} />;
}

/**
 * The context of an analysis page, Skybox style: whose page it is, the scope controls the current view
 * reads and, at the right, what the numbers are compared with and how many maps and rounds they stand
 * on. It sticks under the command strip so the context stays in sight while scrolling.
 */
export function ScopeBar(props: ScopeBarProps): ReactNode {
  return <ScopeFilter {...props} form="header" />;
}
