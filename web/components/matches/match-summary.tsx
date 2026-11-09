'use client';

import type { ReactNode } from 'react';
import type { LibraryDemo, LibraryMatch } from '@/lib/api/library';
import { matchStory, type TeamNames } from '@/lib/library/match-story';
import { teamByKey, teamLabel } from '@/lib/library/partidas';
import { MatchDemoDetails } from '@/components/matches/match-demo-details';
import { MatchScoreboard } from '@/components/matches/match-scoreboard';
import { MatchStoryPanel } from '@/components/matches/match-story-panel';
import { MatchTeamPanel, playerAdr } from '@/components/matches/match-team-panel';
import { useMatchRoundRows } from '@/components/matches/use-match-round-rows';

/** Resumen: the score and every round, how the match went, both teams' players, and the demo it was read from. */
export function MatchSummary({
  match,
  demo,
  pathname,
  onMatchChange,
  onDemoChange,
  onOpenReplay,
}: {
  match: LibraryMatch;
  demo: LibraryDemo;
  pathname: string;
  onMatchChange: (match: LibraryMatch) => void;
  onDemoChange: (demo: LibraryDemo) => void;
  onOpenReplay: () => void;
}): ReactNode {
  const rounds = useMatchRoundRows(match.id);
  const ctStart = teamByKey(match, 'ct-start');
  const tStart = teamByKey(match, 't-start');
  const names: TeamNames = { 'ct-start': teamLabel(ctStart), 't-start': teamLabel(tStart) };
  const facts = rounds.kind === 'ready' ? matchStory(rounds.rows, names) : [];

  let bestAdr: { steamid64: string; adr: number } | null = null;
  for (const player of match.players ?? []) {
    const adr = playerAdr(player);
    if (adr > (bestAdr?.adr ?? 0)) bestAdr = { steamid64: player.steamid64, adr };
  }

  return (
    <div className="flex flex-col gap-5">
      <MatchScoreboard teams={[ctStart, tStart]} names={names} rounds={rounds} pathname={pathname} onOpenRound={onOpenReplay} />
      {facts.length > 0 ? <MatchStoryPanel facts={facts} pathname={pathname} onOpenReplay={onOpenReplay} /> : null}
      <div className="grid gap-5 @[64rem]/content:grid-cols-2">
        {match.teams.map((team) => (
          <MatchTeamPanel
            key={team.key}
            match={match}
            team={team}
            pathname={pathname}
            bestAdr={bestAdr}
            onMatchChange={onMatchChange}
            onOpenReplay={onOpenReplay}
          />
        ))}
      </div>
      <MatchDemoDetails demo={demo} onDemoChange={onDemoChange} />
    </div>
  );
}
