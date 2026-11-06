'use client';

import type { ReactNode } from 'react';
import { useSearchParams } from 'next/navigation';
import { LIBRETA_QUERY, LIBRETA_TABS } from '@/lib/clips/routes';
import { RecordTabs } from '@/components/records/record-tabs';
import { PlayDetection } from '@/components/stratbook/play-detection';
import { useRecordTab } from '@/hooks/use-record-tab';
import { PlayList, useLibretaPlays } from './play-list';

const TABS = Object.values(LIBRETA_TABS);

/** The Libreta: the team's plays and everything the team agrees on, one tab each. */
export function LibretaHub(): ReactNode {
  const search = useSearchParams();
  const requestedTeam = search.get(LIBRETA_QUERY.team) ?? undefined;
  const [tab, setTab] = useRecordTab(TABS, LIBRETA_TABS.plays);
  const { plays, offline, reload } = useLibretaPlays();

  return (
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
              onDetect={() => setTab(LIBRETA_TABS.detect)}
            />
          ),
        },
        {
          id: LIBRETA_TABS.detect,
          label: 'Detectar repetidas',
          content: <PlayDetection initialTeamId={requestedTeam} onSaved={reload} />,
        },
      ]}
    />
  );
}
