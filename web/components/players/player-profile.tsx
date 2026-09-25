'use client';

import Link from 'next/link';
import type { ReactNode } from 'react';
import { Copy, ExternalLink, MoreHorizontal, UploadCloud, UserMinus, UserPlus } from 'lucide-react';
import { toast } from 'sonner';
import type { FaceitFollowedPlayer } from '@/lib/api/faceit';
import { NEW_DEMO_HREF } from '@/lib/clips/routes';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { PlayerAvatar } from './player-avatar';
import { LevelBadge } from './level-badge';

export function PlayerProfile({ player, onUnfollow, unfollowing, onFollow }: {
  player: FaceitFollowedPlayer;
  onUnfollow: () => void;
  unfollowing: boolean;
  /** Present for a seeded zone row: adds the player to the Custom list. */
  onFollow?: () => void;
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
        <PlayerAvatar nickname={player.nickname} playerID={player.id} avatar={player.avatar} size={88} />
        <LevelBadge level={player.skill_level} className="absolute -right-1.5 -bottom-1.5 size-9 text-body-sm ring-3 ring-surface-2" />
      </div>
      <div className="min-w-0 flex-1 basis-40">
        <h2 className="break-words text-section font-bold text-fg-1">{player.nickname}</h2>
        <p className="mt-1 text-body-sm text-fg-2">
          FACEIT · Nivel {player.skill_level ?? '—'}
          {player.country ? <> · <span className="uppercase">{player.country}</span></> : null}
        </p>
        <p className="mt-1 text-stat font-semibold tabular-nums text-fg-1">
          {player.elo ?? '—'} <span className="text-body-sm font-normal text-fg-3">ELO</span>
        </p>
        <div className="mt-1 flex min-w-0 items-center gap-1 text-body-sm text-fg-3">
          <span className="flex min-w-0 gap-1"><span className="shrink-0 whitespace-nowrap">Steam ID</span> <span className="truncate select-all tabular-nums text-fg-2" title={player.steam_id64}>{player.steam_id64 ?? '—'}</span></span>
          {player.steam_id64 ? <Button variant="ghost" size="icon-xs" aria-label="Copiar Steam ID" onClick={() => void copySteamID()}>
            <Copy aria-hidden className="size-3.5" />
          </Button> : null}
        </div>
      </div>
      {/* Full width on phones so the three actions share one row; they still wrap on very narrow screens. */}
      <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto [&>a]:px-2.5">
        <Button asChild variant="outline" size="sm" className="flex-1 sm:flex-none"><a href={player.profile_url} target="_blank" rel="noreferrer">
          Ver perfil FACEIT <ExternalLink aria-hidden className="size-3.5" />
        </a></Button>
        <Button asChild size="sm" className="flex-1 shadow-none sm:flex-none"><Link href={NEW_DEMO_HREF}><UploadCloud aria-hidden className="size-4" /> Subir demo</Link></Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild><Button variant="outline" size="icon-sm" aria-label={`Opciones de ${player.nickname}`} disabled={unfollowing}>
            <MoreHorizontal aria-hidden className="size-4" />
          </Button></DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {onFollow ? <DropdownMenuItem onSelect={onFollow}>
              <UserPlus aria-hidden className="size-4" /> Añadir {player.nickname} a Custom
            </DropdownMenuItem> : null}
            <DropdownMenuItem variant="destructive" onSelect={onUnfollow}>
              <UserMinus aria-hidden className="size-4" />
              {player.seeded === true ? `Quitar a ${player.nickname} de la lista` : `Dejar de seguir a ${player.nickname}`}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
