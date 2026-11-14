'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Bookmark, Check, ListVideo, Plus } from 'lucide-react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { PLAYLISTS_HREF, holdsRound, savedPlaylistHref } from '@/lib/replay/saved-playlist';
import { playlistFolders } from '@/lib/replay/playlist-library';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';
import { PlaylistDialog } from './playlist-dialog';
import type { SavedPlaylistsControl } from './use-saved-playlists';

/**
 * The bookmark on the map's left edge: outlined while the round is in no saved playlist, green and
 * filled once it is in one. It opens the chooser that adds the round to a playlist, removes it or
 * starts a new one. `card` hangs under it, such as the open playlist's comment.
 */
export function PlaylistBookmark({ control, entry, card }: { control: SavedPlaylistsControl; entry: ReplayRef; card?: ReactNode }): ReactNode {
  const [creating, setCreating] = useState(false);
  const { playlists, target } = control;
  const holding = (playlists ?? []).filter((p) => holdsRound(p, entry));
  const inPlaylist = holding.length > 0;
  const label = inPlaylist ? `En playlist: ${holding.map((p) => p.name).join(', ')}` : 'Añadir la ronda a una playlist';
  return (
    <div className="pointer-events-none absolute bottom-2 left-2 z-30 flex max-w-[calc(100%-1rem)] flex-col-reverse items-start gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type="button"
            size="icon-sm"
            variant={inPlaylist ? 'default' : 'outline'}
            disabled={playlists === null}
            aria-label={label}
            title={`${label}. Tecla B`}
            className={cn('pointer-events-auto', inPlaylist ? '[&_svg]:fill-current' : 'bg-surface-1/90 backdrop-blur-sm')}
          >
            <Bookmark aria-hidden />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-72">
          {playlists !== null && playlists.length > 0 ? <DropdownMenuLabel>Añadir la ronda {entry.round} a</DropdownMenuLabel> : null}
          {(playlists ?? []).map((p) => {
            const held = holdsRound(p, entry);
            return (
              <DropdownMenuItem key={p.id} disabled={control.busy} onSelect={() => control.toggleIn(p.id, entry)}>
                <Check aria-hidden className={held ? 'text-primary' : 'invisible'} />
                <span className="min-w-0 flex-1 truncate">{p.name}</span>
                <span className="tabular-nums text-fg-3">{p.rounds.length}</span>
                <span className="sr-only">{held ? ', ya tiene esta ronda; elegirla la quita' : ''}</span>
              </DropdownMenuItem>
            );
          })}
          {playlists !== null && playlists.length > 0 ? <DropdownMenuSeparator /> : null}
          <DropdownMenuItem disabled={control.busy} onSelect={() => setCreating(true)}>
            <Plus aria-hidden />
            Nueva playlist…
          </DropdownMenuItem>
          {target !== undefined && target.rounds.length > 0 ? (
            <DropdownMenuItem asChild>
              <Link href={savedPlaylistHref(target.id)}>
                <ListVideo aria-hidden />
                <span className="min-w-0 flex-1 truncate">Abrir {target.name}</span>
                <span className="tabular-nums text-fg-3">{target.rounds.length}</span>
              </Link>
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem asChild>
            <Link href={PLAYLISTS_HREF}>
              <ListVideo aria-hidden />
              Ver todas las playlists
            </Link>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {control.error ? (
        <p role="status" className="pointer-events-auto w-56 rounded-md border border-border bg-surface-1/95 p-2 text-body-sm text-destructive">
          {control.error}
        </p>
      ) : null}
      {card === undefined ? null : <div className="pointer-events-auto">{card}</div>}
      {creating ? (
        <PlaylistDialog
          title="Nueva playlist"
          description={`Empieza con la ronda ${entry.round}. La tecla B añadirá aquí las siguientes.`}
          submitLabel="Crear playlist"
          folders={playlistFolders(playlists ?? []).map((f) => f.name)}
          onSubmit={async (details) =>
            (await control.create({ name: details.name, folder: details.folder, author: details.author, ref: entry }))
              ? null
              : 'No se pudo crear la playlist. Comprueba que Studio sigue abierto.'
          }
          onClose={() => setCreating(false)}
        />
      ) : null}
    </div>
  );
}
