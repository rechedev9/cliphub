import type { ReactNode } from 'react';
import { notFound } from 'next/navigation';
import { PlayerRecord } from '@/components/players/player-record';

const STEAM_ID_RE = /^[0-9]{1,20}$/;

/** One player's record, by SteamID64. */
export default async function PlayerPage({ params }: { params: Promise<{ steamid: string }> }): Promise<ReactNode> {
  const { steamid } = await params;
  if (!STEAM_ID_RE.test(steamid)) notFound();
  return (
    <div className="measure-work flex flex-col gap-5">
      <PlayerRecord steamId={steamid} />
    </div>
  );
}
