'use client';

import { useId, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ExternalLink, PenLine, X } from 'lucide-react';
import type { ReplayRef } from '@/lib/analysis/replay-link';
import { MAX_PLAYLIST_COMMENT, patchPlaylistRound, removePlaylistRound, type PlaylistRound, type SavedPlaylist } from '@/lib/api/playlists';
import { matchHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { MATCH_REPLAY_TAB, MATCH_TAB_QUERY, TACTICAL_EVIDENCE_QUERY } from '@/lib/library/rounds';
import { countOf, type CountNoun } from '@/lib/records/selection';
import { PLAYLISTS_HREF, playlistRound } from '@/lib/replay/saved-playlist';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import { announcePlaylistChange } from './use-saved-playlists';

function roundKey(round: PlaylistRound): string {
  return `${round.match_id}:${round.round}`;
}

/** The round in its own match's Replay, out of the playlist. */
function roundMatchHref(round: PlaylistRound): string {
  const params = new URLSearchParams({ [MATCH_TAB_QUERY]: MATCH_REPLAY_TAB, [TACTICAL_EVIDENCE_QUERY.round]: String(round.round) });
  return `${matchHref(round.match_id)}?${params.toString()}`;
}

const RONDA: CountNoun = { one: 'ronda', many: 'rondas', feminine: true };
const NOTA: CountNoun = { one: 'nota o dibujo', many: 'notas o dibujos', feminine: true };

/**
 * The open playlist in the Replay's left panel, for the team meeting: the
 * comment of the round on screen, then every round to jump to or take out.
 */
export function SavedPlaylistPanel({
  playlist,
  current,
  labelOf,
  onSelect,
  onChange,
}: {
  playlist: SavedPlaylist;
  current: ReplayRef | undefined;
  /** The round's match as the reader knows it, such as `Mirage · FUT vs OG`. */
  labelOf: (round: PlaylistRound) => string;
  onSelect: (ref: ReplayRef) => void;
  onChange: (next: SavedPlaylist) => void;
}): ReactNode {
  const [removing, setRemoving] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const shown = current === undefined ? undefined : playlistRound(playlist, current);
  const meta = [countOf(playlist.rounds.length, RONDA), playlist.folder, playlist.author].filter((part) => part !== '').join(' · ');

  async function remove(round: PlaylistRound): Promise<void> {
    setRemoving(roundKey(round));
    setError(null);
    try {
      onChange(await removePlaylistRound({ playlistId: playlist.id, matchId: round.match_id, round: round.round }));
      announcePlaylistChange();
    } catch {
      setError('No se pudo quitar la ronda. Comprueba que Studio sigue abierto.');
    } finally {
      setRemoving(null);
    }
  }

  return (
    <section aria-label={`Playlist ${playlist.name}`} className="flex flex-col gap-3">
      <header className="flex flex-col gap-0.5 pr-8">
        <h3 className="truncate font-display text-body font-bold text-fg-1" title={playlist.name}>
          {playlist.name}
        </h3>
        <p className="text-meta text-fg-3">{meta}</p>
        <p className="text-meta text-fg-3">
          <kbd className="font-mono text-fg-2">J</kbd> y <kbd className="font-mono text-fg-2">K</kbd> pasan de ronda.{' '}
          <Link href={PLAYLISTS_HREF} className="text-fg-2 underline decoration-fg-4 underline-offset-2 hover:text-fg-1">
            Todas las playlists
          </Link>
        </p>
      </header>
      {shown === undefined ? null : <RoundComment key={roundKey(shown)} playlist={playlist} round={shown} onChange={onChange} />}
      {error ? (
        <p role="alert" className="text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
      <ol aria-label="Rondas de la playlist" className="flex flex-col gap-0.5">
        {playlist.rounds.map((round, at) => {
          const isCurrent = shown !== undefined && roundKey(shown) === roundKey(round);
          const notes = round.notes?.length ?? 0;
          return (
            <li key={roundKey(round)} className={cn('flex items-start gap-0.5 rounded-md', isCurrent && 'border-l-2 border-l-primary bg-surface-3')}>
              <button
                type="button"
                disabled={round.missing === true}
                aria-current={isCurrent ? 'true' : undefined}
                onClick={() => onSelect({ matchId: round.match_id, round: round.round })}
                className={cn(
                  'flex min-w-0 flex-1 flex-col items-start gap-0.5 rounded-md px-2 py-1.5 text-left outline-none enabled:hover:bg-surface-2 disabled:cursor-default',
                  FOCUS_RING,
                )}
              >
                <span className="flex w-full items-baseline gap-2">
                  <span className="w-5 shrink-0 text-meta tabular-nums text-fg-3">{at + 1}</span>
                  <span className={cn('text-body-sm font-medium', isCurrent ? 'text-primary' : 'text-fg-1')}>Ronda {round.round}</span>
                  {notes > 0 ? (
                    <span className="ml-auto flex items-center gap-0.5 text-meta tabular-nums text-fg-3" title={countOf(notes, NOTA)}>
                      <PenLine aria-hidden className="size-3" />
                      {notes}
                      <span className="sr-only">{notes === 1 ? ' nota' : ' notas'}</span>
                    </span>
                  ) : null}
                </span>
                <span className="w-full truncate pl-7 text-meta text-fg-2">
                  {round.missing ? `${prettyMapName(round.map)} · Falta la demo en esta biblioteca` : labelOf(round)}
                </span>
                {round.comment === '' ? null : <span className="line-clamp-2 w-full pl-7 text-meta text-fg-3">{round.comment}</span>}
              </button>
              <span className="flex shrink-0 flex-col py-1">
                {round.missing ? null : (
                  <Button asChild size="icon-xs" variant="ghost">
                    <Link href={roundMatchHref(round)} aria-label={`Abrir la ronda ${round.round} en su partida`} title="Abrir en su partida">
                      <ExternalLink aria-hidden />
                    </Link>
                  </Button>
                )}
                <Button
                  type="button"
                  size="icon-xs"
                  variant="ghost"
                  disabled={removing !== null}
                  aria-label={`Quitar la ronda ${round.round} de la playlist`}
                  title="Quitar de la playlist"
                  onClick={() => void remove(round)}
                >
                  <X aria-hidden />
                </Button>
              </span>
            </li>
          );
        })}
      </ol>
    </section>
  );
}

type SaveState = 'idle' | 'saving' | 'saved' | 'failed';

const SAVE_COPY: Record<SaveState, string> = {
  idle: 'Se guarda al salir del cuadro.',
  saving: 'Guardando…',
  saved: 'Guardado.',
  failed: 'No se pudo guardar el comentario. Comprueba que Studio sigue abierto.',
};

/** The comment the team reads while the round plays; it saves when the box loses focus. */
function RoundComment({ playlist, round, onChange }: { playlist: SavedPlaylist; round: PlaylistRound; onChange: (next: SavedPlaylist) => void }): ReactNode {
  const [text, setText] = useState(round.comment);
  const [state, setState] = useState<SaveState>('idle');
  const id = useId();

  async function save(): Promise<void> {
    const next = text.trim();
    if (next === round.comment) return;
    setState('saving');
    try {
      onChange(await patchPlaylistRound({ playlistId: playlist.id, matchId: round.match_id, round: round.round }, { comment: next }));
      announcePlaylistChange();
      setState('saved');
    } catch {
      setState('failed');
    }
  }

  return (
    <div className="flex flex-col gap-1.5 rounded-md border border-border bg-surface-2 p-2">
      <label htmlFor={id} className="text-label font-medium text-fg-2">
        Comentario de la ronda {round.round}
      </label>
      <Textarea
        id={id}
        value={text}
        maxLength={MAX_PLAYLIST_COMMENT}
        rows={4}
        placeholder="Qué tiene que ver el equipo en esta ronda"
        onChange={(event) => {
          setText(event.target.value);
          if (state !== 'saving') setState('idle');
        }}
        onBlur={() => void save()}
        className="min-h-20"
      />
      <p role="status" className={cn('text-meta', state === 'failed' ? 'text-destructive' : 'text-fg-3')}>
        {SAVE_COPY[state]}
      </p>
    </div>
  );
}
