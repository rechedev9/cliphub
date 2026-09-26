'use client';

import { useCallback, useEffect, useReducer, useRef, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { Users } from 'lucide-react';
import {
  FACEIT_CODES, FaceitServiceError, followFaceitPlayer, listFollowedFaceitPlayers,
  lookupFaceitPlayer, unfollowFaceitPlayer,
} from '@/lib/api/faceit';
import { FACEIT_NOT_CONFIGURED_CODE, SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { defaultPlayerTab, followedPlayersReducer, inPlayerTab, type PlayerTab } from '@/lib/followed-players';
import { FollowPlayerForm } from '@/components/players/follow-player-form';
import { FollowedPlayerList } from '@/components/players/followed-player-list';
import { PlayerMatches } from '@/components/players/player-matches';
import { PlayerProfile } from '@/components/players/player-profile';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

type LoadState = 'loading' | 'ready' | 'offline' | 'unconfigured';

const FACEIT_UNCONFIGURED_HINT = 'La conexión con FACEIT no está activada en este PC. Puedes cargar una demo descargada desde una sala de FACEIT para empezar a crear.';
const FACEIT_OFFLINE_HINT = 'Servicio local sin conexión.';
// The service refreshes stale ELOs in the background on a list read; poll to pick them up while the page is open.
const ROSTER_POLL_MS = 60_000;
const ROSTER_REFRESHING_POLL_MS = 5_000;
const WORKSPACE_GRID = 'grid min-w-0 items-start gap-5 @[64rem]/content:grid-cols-[19rem_minmax(0,1fr)] @[80rem]/content:gap-6';

export default function PlayersPage(): ReactNode {
  const [state, setState] = useState<LoadState>('loading');
  const [{ players, selectedID }, dispatch] = useReducer(followedPlayersReducer, { players: [], selectedID: null });
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [unfollowingID, setUnfollowingID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [chosenTab, setChosenTab] = useState<PlayerTab | null>(null);
  const [rosterRefreshing, setRosterRefreshing] = useState(false);
  const tab = chosenTab ?? defaultPlayerTab(players);
  // The profile always belongs to the open list: a selection from another tab falls back to this tab's first player.
  const tabPlayers = players.filter((player) => inPlayerTab(player, tab));
  const selected = tabPlayers.find((player) => player.id === selectedID) ?? tabPlayers[0] ?? null;
  const shownID = selected?.id ?? null;
  const playersRef = useRef(players);
  playersRef.current = players;
  // Bumped by every follow/unfollow; a list read that started before one is older than the rail and is dropped.
  const followGeneration = useRef(0);

  const refresh = useCallback(async (background = false) => {
    const generation = followGeneration.current;
    try {
      const listed = await listFollowedFaceitPlayers();
      if (background && generation !== followGeneration.current) return;
      dispatch({ type: 'listed', players: listed.players, updatedAt: listed.updatedAt });
      setRosterRefreshing(listed.refreshing);
      setState(listed.enabled ? 'ready' : 'unconfigured');
      setError(null);
    } catch (err) {
      if (err instanceof FaceitServiceError && err.code === FACEIT_NOT_CONFIGURED_CODE) {
        setState('unconfigured');
        return;
      }
      // A failed poll keeps the list on screen; the next tick tries again.
      if (background) return;
      setState('offline');
      setError(FACEIT_OFFLINE_HINT);
    }
  }, []);

  useEffect(() => { void refresh(); }, [refresh]);

  useEffect(() => {
    if (state !== 'ready') return;
    const poll = (): void => { if (document.visibilityState === 'visible') void refresh(true); };
    const timer = window.setInterval(poll, rosterRefreshing ? ROSTER_REFRESHING_POLL_MS : ROSTER_POLL_MS);
    document.addEventListener('visibilitychange', poll);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', poll);
    };
  }, [state, rosterRefreshing, refresh]);

  useEffect(() => {
    if (shownID === null || state !== 'ready') return;
    const player = playersRef.current.find((candidate) => candidate.id === shownID);
    if (!player) return;
    let cancelled = false;
    void (async () => {
      try {
        const at = Date.now();
        const live = await lookupFaceitPlayer(player.nickname);
        if (cancelled) return;
        dispatch({ type: 'profile', player: live, at });
        if (player.seeded !== true) {
          await followFaceitPlayer(live.nickname);
          followGeneration.current += 1;
        }
      } catch {
        // A failed profile refresh must not hide the saved player or their history.
      }
    })();
    return () => { cancelled = true; };
  }, [shownID, state]);

  async function follow(nickname: string): Promise<boolean> {
    setBusy(true);
    setError(null);
    try {
      const followed = await followFaceitPlayer(nickname);
      followGeneration.current += 1;
      dispatch({ type: 'followed', player: followed });
      setState('ready');
      return true;
    } catch (err) {
      setError(followErrorMessage(err));
      return false;
    } finally {
      setBusy(false);
    }
  }

  async function onFollow(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    const nickname = query.trim();
    if (nickname === '' || busy) return;
    if (await follow(nickname)) {
      setQuery('');
      setChosenTab('custom');
    }
  }

  async function onUnfollow(playerID: string): Promise<void> {
    if (unfollowingID !== null) return;
    setUnfollowingID(playerID);
    setError(null);
    try {
      await unfollowFaceitPlayer(playerID);
      followGeneration.current += 1;
      dispatch({ type: 'unfollowed', id: playerID });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo dejar de seguir al jugador. Vuelve a intentarlo.');
    } finally {
      setUnfollowingID(null);
    }
  }

  let body: ReactNode;
  if (state === 'loading') {
    body = <div role="status" aria-label="Cargando jugadores" className={WORKSPACE_GRID}>
      <Skeleton className="h-80 w-full" />
      <div className="space-y-5"><Skeleton className="h-40 w-full" /><Skeleton className="h-28 w-full" /><Skeleton className="h-80 w-full" /></div>
    </div>;
  } else if (state === 'unconfigured') {
    body = <StudioEmptyState icon={Users} title="FACEIT no está configurado" description={FACEIT_UNCONFIGURED_HINT}
      actions={<Button asChild><Link href="/clips/nueva">Cargar una demo</Link></Button>}
      note={<details className="text-left normal-case tracking-normal">
        <summary className="min-h-10 cursor-pointer py-2">Configuración avanzada de FACEIT</summary>
        <p className="break-words text-body-sm">Configura FACEIT_API_KEY en el servicio local y reinicia ClipHub para consultar perfiles e historial.</p>
      </details>} compact />;
  } else if (players.length === 0 && state === 'offline') {
    body = <StudioEmptyState icon={Users} title="Servicio local sin conexión"
      description="Arranca el servicio local y vuelve a intentarlo para ver a los jugadores que sigues."
      actions={<Button variant="outline" onClick={() => void refresh()}>Reintentar</Button>} compact />;
  } else if (players.length === 0) {
    body = <StudioEmptyState icon={Users} title="Aún no sigues a nadie"
      description="Busca un nick o pega una URL de FACEIT para abrir su historial aquí." compact />;
  } else {
    body = <div className={WORKSPACE_GRID}>
      <FollowedPlayerList players={players} tab={tab} onTabChange={setChosenTab}
        selectedID={shownID} onSelect={(id) => dispatch({ type: 'selected', id })} />
      {selected ? <section aria-label={`Perfil de ${selected.nickname}`} className="studio-panel min-w-0 overflow-hidden">
        <PlayerProfile player={selected} onUnfollow={() => void onUnfollow(selected.id)} unfollowing={unfollowingID !== null}
          onFollow={selected.seeded === true ? () => { if (!busy) void follow(selected.nickname); } : undefined} />
        <PlayerMatches key={selected.id} playerID={selected.id} enabled={state === 'ready'} />
      </section> : null}
    </div>;
  }

  return (
    <div className="measure-work flex min-w-0 flex-col gap-8">
      <StudioPageHeader title="Jugadores" description="Sigue jugadores y convierte sus partidas en clips."
        actions={<FollowPlayerForm query={query} onQueryChange={setQuery} onSubmit={(event) => void onFollow(event)}
          disabled={state !== 'ready'} busy={busy} />} />
      {/* The empty state already says offline with its own Reintentar; a second red line above it is noise. */}
      {error && !(state === 'offline' && players.length === 0) ? <div role="alert" className="flex flex-wrap items-center gap-3 text-body-sm text-destructive">
        <p>{error}</p>
        {state === 'offline' && players.length > 0 ? <Button variant="outline" size="sm" onClick={() => void refresh()}>Reconectar</Button> : null}
      </div> : null}
      {body}
    </div>
  );
}

function followErrorMessage(err: unknown): string {
  if (!(err instanceof FaceitServiceError)) return err instanceof Error ? err.message : 'No se pudo seguir al jugador';
  if (err.code === FACEIT_NOT_CONFIGURED_CODE) return FACEIT_UNCONFIGURED_HINT;
  if (err.code === SERVICE_UNAVAILABLE_CODE) return FACEIT_OFFLINE_HINT;
  if (err.code === FACEIT_CODES.rateLimited) return 'FACEIT está limitando peticiones. Vuelve a intentarlo en unos instantes.';
  if (err.status === 404) return 'Jugador no encontrado. Comprueba el nick o la URL de FACEIT.';
  if (err.status === 409) return 'Has alcanzado el límite de jugadores seguidos.';
  return err.message;
}
