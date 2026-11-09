'use client';

import type { ReactNode } from 'react';
import type { LibraryMatch } from '@/lib/api/library';
import type { AnalysisScope } from '@/lib/analysis/scope';
import type { ReplayForm } from '@/lib/replay/forms';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { TeamGrenades } from '@/components/patterns/team-grenades';
import { ReplayFormTabs } from '@/components/replay/replay-frame';
import { TeamSpatial } from './team-spatial';

/** The forms a team's rounds are read in; one round at a time opens from any figure as a playlist. */
export const TEAM_REPLAY_FORMS = ['calor', 'recorridos', 'granadas'] as const satisfies readonly ReplayForm[];
export type TeamReplayForm = (typeof TEAM_REPLAY_FORMS)[number];

/** The team's Replay: its rounds as a heatmap, as paths or as grenades, through the page's scope. */
export function TeamReplay({
  teamId,
  scope,
  matches,
  form,
  onFormChange,
  onMapChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  matches: readonly LibraryMatch[];
  form: TeamReplayForm;
  onFormChange: (form: TeamReplayForm) => void;
  onMapChange: (map: string) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-3">
      <ReplayFormTabs
        forms={TEAM_REPLAY_FORMS}
        value={form}
        onChange={(next) => {
          const known = TEAM_REPLAY_FORMS.find((entry) => entry === next);
          if (known !== undefined) onFormChange(known);
        }}
      />
      {form === 'granadas' ? (
        <TeamGrenades teamId={teamId} scope={scope} matches={matches} onMapChange={onMapChange} onCounts={onCounts} />
      ) : (
        <TeamSpatial teamId={teamId} scope={scope} view={form === 'calor' ? 'heatmap' : 'overlay'} onMapChange={onMapChange} onCounts={onCounts} />
      )}
    </div>
  );
}
