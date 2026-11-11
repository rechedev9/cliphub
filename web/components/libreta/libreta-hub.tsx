'use client';

import { useState, type ReactNode } from 'react';
import { useSearchParams } from 'next/navigation';
import { LIBRETA_QUERY, LIBRETA_TABS } from '@/lib/clips/routes';
import { RecordTabs } from '@/components/records/record-tabs';
import { DocumentPage } from '@/components/records/document-page';
import { PlayDetection } from '@/components/stratbook/play-detection';
import { useRecordTab } from '@/hooks/use-record-tab';
import { LIBRETA_DESCRIPTION, LIBRETA_TITLE } from '@/lib/libreta/copy';
import { CallList, useLibretaCalls } from './call-list';
import { CalloutSheet } from './callout-sheet';
import { LibretaFileActions } from './libreta-file-actions';
import { LineupList, useLibretaLineups } from './lineup-list';
import { PlanTable } from './plan-table';
import { NO_TEAM } from './play-fields';
import { PlayList, useLibretaPlays } from './play-list';
import { RoleSheet } from './role-sheet';

const TABS = Object.values(LIBRETA_TABS);

/** The Libreta: the team's plays and everything the team agrees on, one tab each. */
export function LibretaHub(): ReactNode {
  const search = useSearchParams();
  const requestedTeam = search.get(LIBRETA_QUERY.team) ?? undefined;
  const [tab, setTab] = useRecordTab(TABS, LIBRETA_TABS.plays);
  const { plays, offline, reload } = useLibretaPlays();
  const lineups = useLibretaLineups();
  const calls = useLibretaCalls();
  // Bumped by an import so the tabs that read on mount, Roles and Callouts, read again.
  const [imports, setImports] = useState(0);
  const reloadAll = (): void => {
    reload();
    lineups.reload();
    calls.reload();
    setImports((n) => n + 1);
  };

  return (
    <DocumentPage title={LIBRETA_TITLE} description={LIBRETA_DESCRIPTION} actions={<LibretaFileActions onImported={reloadAll} />}>
      <RecordTabs
        value={tab}
        onValueChange={setTab}
        tabs={[
          {
            id: LIBRETA_TABS.plays,
            label: 'Jugadas',
            count: plays?.filter((play) => play.status !== 'archived').length,
            content: (
              <PlayList
                plays={plays}
                offline={offline}
                requestedTeam={requestedTeam}
                onReload={reload}
                onImported={reloadAll}
                onDetect={() => setTab(LIBRETA_TABS.detect)}
              />
            ),
          },
          {
            id: LIBRETA_TABS.table,
            label: 'Tabla',
            content: <PlanTable plays={plays} onReload={reload} />,
          },
          {
            id: LIBRETA_TABS.lineups,
            label: 'Lineups',
            count: lineups.lineups?.filter((lineup) => lineup.status !== 'archived').length,
            content: <LineupList lineups={lineups.lineups} offline={lineups.offline} onReload={lineups.reload} />,
          },
          {
            id: LIBRETA_TABS.roles,
            label: 'Roles',
            content: <RoleSheet key={imports} plays={plays} />,
          },
          {
            id: LIBRETA_TABS.calls,
            label: 'Llamadas',
            count: calls.calls?.filter((call) => call.status !== 'archived').length,
            content: <CallList calls={calls.calls} offline={calls.offline} onReload={calls.reload} />,
          },
          {
            id: LIBRETA_TABS.callouts,
            label: 'Callouts',
            content: <CalloutSheet key={imports} plays={plays} lineups={lineups.lineups} />,
          },
          {
            id: LIBRETA_TABS.detect,
            label: 'Detectar repetidas',
            content: <PlayDetection initialTeamId={requestedTeam === NO_TEAM ? undefined : requestedTeam} onSaved={reload} />,
          },
        ]}
      />
    </DocumentPage>
  );
}
