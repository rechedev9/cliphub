'use client';

import { useState, type ReactNode } from 'react';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { prettyMapName } from '@/lib/format';
import { AttackPrep } from '@/components/prep/attack-sections';
import { Button } from '@/components/ui/button';

/** The T side match prep of a saved report, map by map, over the report's own matches. */
export function ReportAttack({ teamId, matches }: { teamId: string; matches: readonly ScoutMatchSummary[] }): ReactNode {
  const maps = [...new Set(matches.map((match) => match.map))];
  const [map, setMap] = useState(maps[0] ?? '');
  const matchIds = matches.filter((match) => match.map === map).map((match) => match.match_id);
  return (
    <div className="flex flex-col gap-5">
      {maps.length < 2 ? null : (
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Mapa del ataque">
          {maps.map((entry) => (
            <Button key={entry} type="button" size="xs" variant={entry === map ? 'outline' : 'ghost'} aria-pressed={entry === map} onClick={() => setMap(entry)}>
              {prettyMapName(entry)}
            </Button>
          ))}
        </div>
      )}
      {map === '' ? null : <AttackPrep query={{ teamId, map, matchIds }} />}
    </div>
  );
}
