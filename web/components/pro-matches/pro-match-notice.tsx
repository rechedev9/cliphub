'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Trophy, TriangleAlert } from 'lucide-react';
import { getMatchPro, type ProMatchInfo } from '@/lib/api/pro-matches';
import { PRO_MATCHES_HREF } from '@/lib/clips/routes';
import { RADAR_OFF_NOTE } from './pro-match-card';

/** The HLTV match a pro import came from, and a warning when its radar may draw off. */
export function ProMatchNotice({ matchId }: { matchId: string }): ReactNode {
  const [info, setInfo] = useState<ProMatchInfo | null>(null);

  useEffect(() => {
    let active = true;
    getMatchPro(matchId)
      .then((next) => active && setInfo(next))
      .catch(() => active && setInfo(null));
    return () => {
      active = false;
    };
  }, [matchId]);

  if (info === null) return null;
  return (
    <div className="studio-panel flex flex-col gap-1.5 px-4 py-3 text-body-sm text-fg-2">
      <p className="flex items-center gap-2">
        <Trophy aria-hidden className="size-4 shrink-0 text-fg-3" />
        <span>
          Partida pro de HLTV{info.event ? `, ${info.event}` : ''}. Llegó analizada desde{' '}
          <Link href={PRO_MATCHES_HREF} className="text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
            Partidas pro
          </Link>
          , sin la demo.
        </span>
      </p>
      {info.radar_may_be_off ? (
        <p className="flex items-start gap-2 text-warning">
          <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0" />
          {RADAR_OFF_NOTE}
        </p>
      ) : null}
    </div>
  );
}
