'use client';

import { useState, type ReactNode } from 'react';
import { useSearchParams } from 'next/navigation';
import { LIBRETA_QUERY, LIBRETA_TAB_LABEL, LIBRETA_TABS } from '@/lib/clips/routes';
import { RecordTabs } from '@/components/records/record-tabs';
import { PlayDetection } from '@/components/stratbook/play-detection';
import { useRecordTab } from '@/hooks/use-record-tab';
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
    <div className="flex flex-col gap-3">
      <LibretaFileActions onImported={reloadAll} />
      <RecordTabs
        value={tab}
        onValueChange={setTab}
        tabs={[
          {
            id: LIBRETA_TABS.plays,
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.plays],
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
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.table],
            content: <PlanTable plays={plays} onReload={reload} />,
          },
          {
            id: LIBRETA_TABS.lineups,
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.lineups],
            count: lineups.lineups?.filter((lineup) => lineup.status !== 'archived').length,
            content: <LineupList lineups={lineups.lineups} offline={lineups.offline} onReload={lineups.reload} />,
          },
          {
            id: LIBRETA_TABS.roles,
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.roles],
            content: <RoleSheet key={imports} plays={plays} />,
          },
          {
            id: LIBRETA_TABS.calls,
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.calls],
            count: calls.calls?.filter((call) => call.status !== 'archived').length,
            content: <CallList calls={calls.calls} offline={calls.offline} onReload={calls.reload} />,
          },
          {
            id: LIBRETA_TABS.callouts,
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.callouts],
            content: <CalloutSheet key={imports} plays={plays} lineups={lineups.lineups} />,
          },
          {
            id: LIBRETA_TABS.detect,
            label: LIBRETA_TAB_LABEL[LIBRETA_TABS.detect],
            content: <PlayDetection initialTeamId={requestedTeam === NO_TEAM ? undefined : requestedTeam} onSaved={reload} />,
          },
        ]}
      />
    </div>
  );
}
