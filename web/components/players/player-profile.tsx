'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { Copy, ExternalLink, MoreHorizontal, UploadCloud, UserMinus } from 'lucide-react';
import { toast } from 'sonner';
import type { FaceitFollowedPlayer } from '@/lib/api/faceit';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { PlayerAvatar } from './player-avatar';
import { LevelBadge } from './level-badge';

export function PlayerProfile({ player, onUnfollow, unfollowing }: {
  player: FaceitFollowedPlayer;
  onUnfollow: () => void;
  unfollowing: boolean;
}): ReactNode {
  async function copySteamID(): Promise<void> {
    if (!player.steam_id64) return;
    try {
      await navigator.clipboard.writeText(player.steam_id64);
      toast.success('Steam ID copiado');
    } catch {
      toast.error('No se pudo copiar el Steam ID. Selecciona el número para copiarlo.');
    }
  }

  return (
    <header className="flex flex-wrap items-center gap-5 border-b border-border p-4 sm:p-5">
      <div className="relative shrink-0">
        <PlayerAvatar nickname={player.nickname} playerID={player.id} size={88} />
        <LevelBadge level={player.skill_level} className="absolute -right-1 -bottom-1 size-8 text-body-sm" />
      </div>
      <div className="min-w-0 flex-1 basis-40">
        <h2 className="break-words text-section font-bold text-fg-1">{player.nickname}</h2>
        <p className="mt-1 text-body-sm text-fg-2">
          FACEIT · Nivel {player.skill_level ?? '—'}
          {player.country ? <span className="ml-2 uppercase text-fg-3">{player.country}</span> : null}
        </p>
        <p className="mt-1 text-stat font-semibold tabular-nums text-fg-1">
          {player.elo ?? '—'} <span className="text-body-sm font-normal text-fg-3">ELO</span>
        </p>
        <div className="mt-1 flex min-w-0 items-center gap-1 text-body-sm text-fg-3">
          <span className="break-all">Steam ID <span className="select-all text-fg-2">{player.steam_id64 ?? '—'}</span></span>
          {player.steam_id64 ? <Button variant="ghost" size="icon-xs" aria-label="Copiar Steam ID" onClick={() => void copySteamID()}>
            <Copy aria-hidden className="size-3.5" />
          </Button> : null}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2 [&>a]:px-2.5">
        <Button asChild variant="outline" size="sm"><a href={player.profile_url} target="_blank" rel="noreferrer">
          Ver perfil FACEIT <ExternalLink aria-hidden className="size-3.5" />
        </a></Button>
        <Button asChild size="sm" className="shadow-none"><Link href="/upload"><UploadCloud aria-hidden className="size-4" /> Subir demo</Link></Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild><Button variant="outline" size="icon-sm" aria-label={`Opciones de ${player.nickname}`} disabled={unfollowing}>
            <MoreHorizontal aria-hidden className="size-4" />
          </Button></DropdownMenuTrigger>
          <DropdownMenuContent align="end"><DropdownMenuItem variant="destructive" onSelect={onUnfollow}>
            <UserMinus aria-hidden className="size-4" /> Dejar de seguir a {player.nickname}
          </DropdownMenuItem></DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
