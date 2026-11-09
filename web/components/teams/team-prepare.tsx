'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Printer, TriangleAlert } from 'lucide-react';
import type { LibraryMatch, LibraryTeam } from '@/lib/api/library';
import type { ScoutReport } from '@/lib/api/scout';
import { prepareWarnings } from '@/lib/analysis/prepare-warnings';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { prepHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { REPLAY_FORM_LABEL } from '@/lib/replay/forms';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { PrepBody } from '@/components/prep/prep-body';
import { Button } from '@/components/ui/button';
import { TEAM_REPLAY_FORMS, type TeamReplayForm } from './team-replay';
import { TeamScout } from './team-scout';

/**
 * Preparar: everything to get ready for a rival in one tab. First what to
 * distrust (an old sample, a lineup that changed), then the map's preparation
 * (buys, tempo, CT positions, playbooks) and under it the scouting sections.
 * Each part opens the Replay form that shows it.
 */
export function TeamPrepare({
  team,
  matches,
  maps,
  scope,
  lineup,
  onMapChange,
  onOpenReplay,
  onCounts,
}: {
  team: LibraryTeam;
  /** The team's matches the scope keeps. */
  matches: readonly LibraryMatch[];
  maps: readonly string[];
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  onMapChange: (map: string) => void;
  onOpenReplay: (form: TeamReplayForm) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);
  const [kept, setKept] = useState<readonly string[] | null>(null);
  const onReport = useCallback((report: ScoutReport | null) => setKept(report === null ? null : report.matches.map((match) => match.played_at)), []);
  const warnings = useMemo(
    () => prepareWarnings({ roster: team.roster ?? [], periodPlayedAt: matches.map((match) => match.played_at), keptPlayedAt: kept, now }),
    [team.roster, matches, kept, now],
  );
  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Abrir en el Replay">
          <span className="text-body-sm text-fg-2">Abrir en el Replay</span>
          {TEAM_REPLAY_FORMS.map((form) => (
            <Button key={form} type="button" variant="outline" size="xs" onClick={() => onOpenReplay(form)}>
              {REPLAY_FORM_LABEL[form]}
            </Button>
          ))}
        </div>
        {scope.map === '' ? null : (
          <Button asChild variant="ghost" size="xs">
            <Link href={prepHref({ teamId: team.id, map: scope.map, from: from?.slice(0, 10), to: to?.slice(0, 10), lineup })}>
              <Printer aria-hidden />
              Imprimir la preparación
            </Link>
          </Button>
        )}
      </div>
      {warnings.length === 0 ? null : (
        <ul className="flex flex-col gap-1 rounded-md border border-border-strong bg-surface-2 px-3 py-2">
          {warnings.map((warning) => (
            <li key={warning} className="flex items-start gap-2 text-body-sm text-fg-1">
              <TriangleAlert className="mt-0.5 size-4 shrink-0 text-fg-2" aria-hidden />
              {warning}
            </li>
          ))}
        </ul>
      )}
      {scope.map === '' ? (
        <div className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-surface-1 px-3 py-2.5">
          <p className="text-body-sm text-fg-2">Elige un mapa para ver compras contra compras, ritmo, posiciones de CT y playbooks:</p>
          {maps.map((map) => (
            <Button key={map} type="button" variant="outline" size="xs" onClick={() => onMapChange(map)}>
              {prettyMapName(map)}
            </Button>
          ))}
        </div>
      ) : (
        <PrepBody query={{ teamId: team.id, map: scope.map, from, to, lineup }} onOpenReplay={(form) => {
            const known = TEAM_REPLAY_FORMS.find((entry) => entry === form);
            if (known !== undefined) onOpenReplay(known);
          }} />
      )}
      <section aria-label="Scouting" className="flex flex-col gap-2 border-t border-border pt-4">
        <h2 className="text-body font-semibold text-fg-1">Cómo juegan</h2>
        <TeamScout teamId={team.id} scope={scope} lineup={lineup} onCounts={onCounts} onReport={onReport} />
      </section>
    </div>
  );
}
