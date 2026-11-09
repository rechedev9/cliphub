'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Check, ChevronDown, ListPlus, ListVideo, Plus } from 'lucide-react';
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
import { PlaylistDialog } from './playlist-dialog';
import type { SavedPlaylistsControl } from './use-saved-playlists';

/** "Añadir a playlist" of the round panel: puts the round in a saved playlist, or starts one with it. */
export function PlaylistPicker({ control, entry }: { control: SavedPlaylistsControl; entry: ReplayRef }): ReactNode {
  const [creating, setCreating] = useState(false);
  const { playlists, target } = control;
  const inTarget = target !== undefined && holdsRound(target, entry);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" size="xs" variant="outline" aria-pressed={inTarget} disabled={playlists === null} title="Tecla B: añade la ronda a la playlist marcada o la quita">
            <ListPlus aria-hidden />
            <span className="max-w-48 truncate">{inTarget ? `En ${target.name}` : 'Añadir a playlist'}</span>
            <ChevronDown aria-hidden />
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
          <DropdownMenuItem asChild>
            <Link href={PLAYLISTS_HREF}>
              <ListVideo aria-hidden />
              Ver todas las playlists
            </Link>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {target !== undefined && target.rounds.length > 0 ? (
        <Button asChild size="xs" variant="ghost">
          <Link href={savedPlaylistHref(target.id)}>
            <ListVideo aria-hidden />
            <span className="max-w-48 truncate">Abrir {target.name}</span> <span className="tabular-nums text-fg-3">{target.rounds.length}</span>
          </Link>
        </Button>
      ) : null}
      {control.error ? (
        <p role="status" className="basis-full text-body-sm text-destructive">
          {control.error}
        </p>
      ) : null}
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
    </>
  );
}
