'use client';

import { useMemo, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { teamLibretaHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { PlayList, useLibretaPlays } from '@/components/libreta/play-list';

/**
 * "Gestionar": the team's Libreta plays on the Playbook's map and side, with the Libreta's own
 * table, so a play is renamed, archived, exported or deleted without leaving the team.
 */
export function ManagePlays({ teamId, map, side }: { teamId: string; map: string; side: string }): ReactNode {
  const router = useRouter();
  const { plays, offline, reload } = useLibretaPlays();
  const mine = useMemo(() => plays?.filter((play) => play.team_id === teamId && play.map === map && play.side === side) ?? null, [plays, teamId, map, side]);
  return (
    <div className="flex flex-col gap-3">
      <p className="text-body-sm text-fg-2">
        Las jugadas de la Libreta de este equipo en {prettyMapName(map)}, de {side}. Las tácticas de la pestaña Tácticas salen de las rondas; guarda las que quieras conservar.
      </p>
      <PlayList
        plays={mine}
        offline={offline}
        tableId="team-playbook-plays"
        onReload={reload}
        onImported={reload}
        onDetect={() => router.push(teamLibretaHref(teamId, true))}
      />
    </div>
  );
}
