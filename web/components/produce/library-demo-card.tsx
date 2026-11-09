'use client';

import type { ReactNode } from 'react';
import { MATCH_ROW_UNPICKED_CTA, MATCH_ROW_UNPICKED_HINT } from '@/lib/clips/copy';
import type { ProduceFormat } from '@/lib/clips/routes';
import { matchDateLabel } from '@/lib/format';
import { libraryRowTeams, type LibraryDemoRow } from '@/lib/produce/library-rows';
import { cn } from '@/lib/utils';
import { OpenInCrearButton } from '@/components/matches/open-in-crear-button';
import { DemoCardFrame, DemoCover } from '@/components/produce/demo-card';
import { FORMAT_TONE } from '@/components/studio/format-tone';

/** A demo analysed in Partidas that has no Crear job yet: picking the player starts one. */
export function LibraryDemoCard({ row, format }: { row: LibraryDemoRow; format: ProduceFormat }): ReactNode {
  const { title, score } = libraryRowTeams(row.match);
  return (
    <DemoCardFrame>
      <DemoCover map={row.match.map} dateLabel={matchDateLabel({ playedAt: row.demo.imported_at, source: 'upload' })} score={score} />
      <div className="flex flex-1 flex-col gap-2.5 px-4 pt-3.5 pb-4">
        {title === '' ? null : <p className="text-body-sm wrap-anywhere text-fg-2">{title}</p>}
        <span className="text-body-sm text-fg-2">{MATCH_ROW_UNPICKED_HINT}</span>
        <div className="mt-auto flex flex-wrap items-center gap-2">
          <OpenInCrearButton request={{ demo: row.demo, format }} size="sm" variant="outline" className={cn('w-full truncate border-[1.5px] bg-transparent font-bold', FORMAT_TONE[format].border)}>
            {MATCH_ROW_UNPICKED_CTA}
          </OpenInCrearButton>
        </div>
      </div>
    </DemoCardFrame>
  );
}
