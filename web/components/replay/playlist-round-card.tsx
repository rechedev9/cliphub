'use client';

import { useId, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ChevronDown, ExternalLink, SendHorizontal, Trash2 } from 'lucide-react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { MAX_PLAYLIST_COMMENT, patchPlaylistRound, removePlaylistRound, type PlaylistRound, type SavedPlaylist } from '@/lib/api/playlists';
import { PLAYLISTS_HREF, playlistRound } from '@/lib/replay/saved-playlist';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { announcePlaylistChange } from './use-saved-playlists';

function roundKey(round: PlaylistRound): string {
  return `${round.match_id}:${round.round}`;
}

type SaveState = 'idle' | 'saving' | 'saved' | 'failed';

const SAVE_COPY: Record<SaveState, string> = {
  idle: 'Se guarda al enviar o al salir del cuadro.',
  saving: 'Guardando…',
  saved: 'Guardado.',
  failed: 'No se pudo guardar el comentario. Comprueba que Studio sigue abierto.',
};

/**
 * The card over the map for the round on screen: the playlist's name, take the
 * round out, open the playlists, fold, and the comment the team reads while it plays.
 */
export function PlaylistRoundCard({ playlist, current, onChange }: { playlist: SavedPlaylist; current: ReplayRef | undefined; onChange: (next: SavedPlaylist) => void }): ReactNode {
  const round = current === undefined ? undefined : playlistRound(playlist, current);
  if (round === undefined) return null;
  return <RoundCard key={roundKey(round)} playlist={playlist} round={round} onChange={onChange} />;
}

function RoundCard({ playlist, round, onChange }: { playlist: SavedPlaylist; round: PlaylistRound; onChange: (next: SavedPlaylist) => void }): ReactNode {
  const [open, setOpen] = useState(true);
  const [text, setText] = useState(round.comment);
  const [state, setState] = useState<SaveState>('idle');
  const [removing, setRemoving] = useState(false);
  const id = useId();

  async function save(): Promise<void> {
    const next = text.trim();
    if (next === round.comment || state === 'saving') return;
    setState('saving');
    try {
      onChange(await patchPlaylistRound({ playlistId: playlist.id, matchId: round.match_id, round: round.round }, { comment: next }));
      announcePlaylistChange();
      setState('saved');
    } catch {
      setState('failed');
    }
  }

  async function remove(): Promise<void> {
    setRemoving(true);
    try {
      onChange(await removePlaylistRound({ playlistId: playlist.id, matchId: round.match_id, round: round.round }));
      announcePlaylistChange();
    } catch {
      setState('failed');
      setRemoving(false);
    }
  }

  return (
    <section aria-label={`Comentario de la ronda ${round.round} en ${playlist.name}`} className="flex w-56 max-w-full flex-col gap-1.5 rounded-md border border-border bg-surface-1/95 p-2 shadow-md backdrop-blur-sm">
      <header className="flex min-w-0 items-center gap-0.5">
        <h3 className="min-w-0 flex-1 truncate text-meta font-semibold tracking-wide text-fg-1 uppercase" title={playlist.name}>
          {playlist.name}
        </h3>
        <Button type="button" size="icon-xs" variant="ghost" disabled={removing} aria-label={`Quitar la ronda ${round.round} de la playlist`} title="Quitar de la playlist" onClick={() => void remove()}>
          <Trash2 aria-hidden />
        </Button>
        <Button asChild size="icon-xs" variant="ghost">
          <Link href={PLAYLISTS_HREF} aria-label="Ver todas las playlists" title="Ver todas las playlists">
            <ExternalLink aria-hidden />
          </Link>
        </Button>
        <Button type="button" size="icon-xs" variant="ghost" aria-expanded={open} aria-label={open ? 'Plegar el comentario' : 'Desplegar el comentario'} onClick={() => setOpen((value) => !value)}>
          <ChevronDown aria-hidden className={cn('transition-transform duration-(--dur-fast)', !open && '-rotate-90')} />
        </Button>
      </header>
      {open ? (
        <>
          <label htmlFor={id} className="sr-only">
            Comentario de la ronda {round.round}
          </label>
          <div className="flex items-end gap-1">
            <Textarea
              id={id}
              value={text}
              maxLength={MAX_PLAYLIST_COMMENT}
              rows={3}
              placeholder="Añade un comentario"
              onChange={(event) => {
                setText(event.target.value);
                if (state !== 'saving') setState('idle');
              }}
              onBlur={() => void save()}
              className="min-h-16 min-w-0 flex-1"
            />
            <Button type="button" size="icon-xs" variant="ghost" disabled={state === 'saving'} aria-label="Guardar el comentario" title="Guardar el comentario" onClick={() => void save()}>
              <SendHorizontal aria-hidden />
            </Button>
          </div>
          <p role="status" className={cn('text-meta', state === 'failed' ? 'text-destructive' : 'text-fg-3')}>
            {SAVE_COPY[state]}
          </p>
        </>
      ) : null}
    </section>
  );
}
