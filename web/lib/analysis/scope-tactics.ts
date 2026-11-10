import { prettyMapName } from '../format.ts';
import type { LibraryPlay } from '../api/stratbook.ts';
import type { AnalysisScope } from './scope.ts';

/** A Libreta play the Replay's tactic filter can pick, with the map and side it was read on. */
export type TacticOption = { readonly value: string; readonly label: string; readonly map: string; readonly side: string };

type PlayFields = Pick<LibraryPlay, 'id' | 'title' | 'map' | 'side' | 'status'>;

/** Every play of the team, archived ones included, labelled with its map. */
export function tacticOptions(plays: readonly PlayFields[]): TacticOption[] {
  return plays
    .map((play) => ({
      value: play.id,
      label: `${play.title} · ${prettyMapName(play.map)}${play.status === 'archived' ? ' (archivada)' : ''}`,
      map: play.map,
      side: play.side,
    }))
    .sort((a, b) => a.label.localeCompare(b.label, 'es'));
}

/** The plays of the scope's map and side; an empty map or side keeps every one. */
export function tacticChoices(options: readonly TacticOption[], scope: Pick<AnalysisScope, 'map' | 'side'>): TacticOption[] {
  return options.filter((option) => (scope.map === '' || option.map === scope.map) && (scope.side === '' || option.side === scope.side));
}

/** The scope patch for a new map: a tactic read on another map is cleared with it. */
export function mapPatch(input: { map: string; tactic: string; options: readonly TacticOption[] }): Partial<AnalysisScope> {
  const play = input.options.find((option) => option.value === input.tactic);
  if (input.map !== '' && play !== undefined && play.map !== input.map) return { map: input.map, tactic: '' };
  return { map: input.map };
}

/** True when the scope names a tactic that is not among the team's plays, such as a deleted one. */
export function unknownTactic(tactic: string, options: readonly TacticOption[]): boolean {
  return tactic !== '' && !options.some((option) => option.value === tactic);
}
