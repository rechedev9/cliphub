'use client';

import { useCallback, useEffect, useMemo, useReducer, useRef, useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { Users } from 'lucide-react';
import {
  FACEIT_CODES, FaceitServiceError, followFaceitPlayer, getFaceitRanking, listFaceitCountries, listFollowedFaceitPlayers,
  lookupFaceitPlayer, unfollowFaceitPlayer, type FaceitFollowedPlayer, type FaceitPlayer,
} from '@/lib/api/faceit';
import { FACEIT_NOT_CONFIGURED_CODE, SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { getDesktopSettingsBridge } from '@/lib/desktop-settings';
import {
  EMPTY_RANKING_LIST, RANKING_CHOICE_KEY, RANKING_PAGE, initialRankingChoice, parseRankingChoice, rankingChoiceLabel,
  rankingListReducer, rankingOptions, sameRankingChoice, spanishCountryName, type RankingChoice,
} from '@/lib/faceit-ranking';
import { defaultPlayerTab, followedPlayersReducer, headerFollowFailure, profileFollowFailure, type FollowFailure, type PlayerTab } from '@/lib/followed-players';
import { FollowPlayerForm } from '@/components/players/follow-player-form';
import { PlayerMatches } from '@/components/players/player-matches';
import { PlayerProfile } from '@/components/players/player-profile';
import { PlayerRail } from '@/components/players/player-rail';
import { PlayerRows } from '@/components/players/player-rows';
import { RankingPanel } from '@/components/players/ranking-panel';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';

type LoadState = 'loading' | 'ready' | 'offline' | 'unconfigured';

const FACEIT_UNCONFIGURED_HINT = 'La conexión con FACEIT no está activada en este PC. Puedes cargar una demo descargada desde una sala de FACEIT para empezar a crear.';
const FACEIT_OFFLINE_HINT = 'Servicio local sin conexión.';
// The service refreshes the followed ELOs once when Studio starts; while that runs, re-read the list to pick up the result.
const ROSTER_REFRESHING_POLL_MS = 5_000;
const WORKSPACE_GRID = 'grid min-w-0 items-start gap-5 @[64rem]/content:grid-cols-[19rem_minmax(0,1fr)] @[80rem]/content:gap-6';

function savedRankingChoice(): RankingChoice | null {
  try {
    return parseRankingChoice(window.localStorage.getItem(RANKING_CHOICE_KEY));
  } catch {
    return null;
  }
}

async function systemCountry(): Promise<string | undefined> {
  try {
    const info = await getDesktopSettingsBridge()?.getAppInfo();
    return typeof info?.localeCountry === 'string' ? info.localeCountry : undefined;
  } catch {
    return undefined;
  }
}

export default function FaceitPlayersPage(): ReactNode {
  const [state, setState] = useState<LoadState>('loading');
  const [{ players, selectedID }, dispatch] = useReducer(followedPlayersReducer, { players: [], selectedID: null });
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [unfollowingID, setUnfollowingID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [followFailure, setFollowFailure] = useState<FollowFailure | null>(null);
  const [chosenTab, setChosenTab] = useState<PlayerTab | null>(null);
  const [rosterRefreshing, setRosterRefreshing] = useState(false);
  const [countries, setCountries] = useState<string[]>([]);
  const [choice, setChoice] = useState<RankingChoice | null>(null);
  const [ranking, dispatchRanking] = useReducer(rankingListReducer, EMPTY_RANKING_LIST);
  const [rankingSelectedID, setRankingSelectedID] = useState<string | null>(null);
  // Live FACEIT profiles of ranking players, whose ranking row carries no avatar or SteamID.
  const [live, setLive] = useState<Record<string, FaceitPlayer>>({});
  const tab = chosenTab ?? defaultPlayerTab(players);
  const options = useMemo(() => rankingOptions(countries, spanishCountryName), [countries]);
  const followedIDs = useMemo(() => new Set(players.map((player) => player.id)), [players]);

  const followedSelected = players.find((player) => player.id === selectedID) ?? players[0] ?? null;
  const rankingSelected = ranking.players.find((player) => player.id === rankingSelectedID) ?? ranking.players[0] ?? null;
  let shown: FaceitFollowedPlayer | null = followedSelected;
  if (tab === 'ranking') {
    shown = rankingSelected === null ? null
      : players.find((player) => player.id === rankingSelected.id) ?? { ...rankingSelected, ...live[rankingSelected.id] };
  }
  const shownID = shown?.id ?? null;
  const shownRef = useRef(shown);
  shownRef.current = shown;
  const playersRef = useRef(players);
  playersRef.current = players;
  // Bumped by every follow/unfollow; a list read that started before one is older than the rail and is dropped.
  const followGeneration = useRef(0);

  const refresh = useCallback(async (background = false) => {
    const generation = followGeneration.current;
    try {
      const listed = await listFollowedFaceitPlayers();
      if (background && generation !== followGeneration.current) return;
      dispatch({ type: 'listed', players: listed.players });
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
    let cancelled = false;
    void (async () => {
      const [listed, country] = await Promise.all([listFaceitCountries().catch(() => []), systemCountry()]);
      if (cancelled) return;
      setCountries(listed);
      setChoice(initialRankingChoice({ saved: savedRankingChoice(), systemCountry: country, countries: listed }));
    })();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (state !== 'ready' || !rosterRefreshing) return;
    const timer = window.setInterval(() => { void refresh(true); }, ROSTER_REFRESHING_POLL_MS);
    return () => { window.clearInterval(timer); };
  }, [state, rosterRefreshing, refresh]);

  const loadRankingPage = useCallback(async (target: RankingChoice, offset: number) => {
    try {
      const page = await getFaceitRanking({ ...target, offset, limit: RANKING_PAGE });
      dispatchRanking({ type: 'page', choice: target, offset, page });
    } catch (err) {
      dispatchRanking({ type: 'failed', choice: target, offset, code: err instanceof FaceitServiceError ? err.code : 'unknown' });
    }
  }, []);

  // The ranking is read only while its tab is open: each new choice costs FACEIT requests.
  useEffect(() => {
    if (state !== 'ready' || tab !== 'ranking' || choice === null || sameRankingChoice(ranking.choice, choice)) return;
    dispatchRanking({ type: 'choose', choice });
    void loadRankingPage(choice, 0);
  }, [state, tab, choice, ranking.choice, loadRankingPage]);

  useEffect(() => {
    if (shownID === null || state !== 'ready') return;
    const player = shownRef.current;
    if (player === null) return;
    let cancelled = false;
    void (async () => {
      try {
        const fresh = await lookupFaceitPlayer(player.nickname);
        if (cancelled) return;
        setLive((profiles) => ({ ...profiles, [fresh.id]: fresh }));
        if (playersRef.current.some((followed) => followed.id === fresh.id)) {
          dispatch({ type: 'profile', player: fresh });
          await followFaceitPlayer(fresh.nickname);
          followGeneration.current += 1;
        }
      } catch {
        // A failed profile refresh must not hide the saved player or their history.
      }
    })();
    return () => { cancelled = true; };
  }, [shownID, state]);

  function chooseRanking(next: RankingChoice): void {
    setChoice(next);
    setRankingSelectedID(null);
    try {
      window.localStorage.setItem(RANKING_CHOICE_KEY, JSON.stringify(next));
    } catch {
      // A full or blocked storage only costs remembering the choice.
    }
  }

  function retryRanking(): void {
    if (choice === null) return;
    dispatchRanking({ type: 'choose', choice });
    void loadRankingPage(choice, 0);
  }

  function loadMoreRanking(): void {
    if (ranking.choice === null || ranking.loadingMore) return;
    dispatchRanking({ type: 'more' });
    void loadRankingPage(ranking.choice, ranking.players.length);
  }

  async function follow(nickname: string, profileID?: string): Promise<boolean> {
    // The first follow or the last unfollow would otherwise move the default tab under the user.
    setChosenTab(tab);
    setBusy(true);
    setError(null);
    setFollowFailure(null);
    try {
      const followed = await followFaceitPlayer(nickname);
      followGeneration.current += 1;
      dispatch({ type: 'followed', player: followed });
      setState('ready');
      return true;
    } catch (err) {
      setFollowFailure({ message: followErrorMessage(err), profileID });
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
      setChosenTab('seguidos');
    }
  }

  async function onUnfollow(playerID: string): Promise<void> {
    if (unfollowingID !== null) return;
    setChosenTab(tab);
    setUnfollowingID(playerID);
    setError(null);
    setFollowFailure(null);
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
      actions={<Button asChild><Link href="/shorts">Cargar una demo</Link></Button>}
      note={<details className="text-left normal-case tracking-normal">
        <DisclosureSummary className="min-h-10 py-2">Configuración avanzada de FACEIT</DisclosureSummary>
        <p className="break-words text-body-sm">Configura FACEIT_API_KEY en el servicio local y reinicia ClipHub para consultar perfiles e historial.</p>
      </details>} compact />;
  } else if (players.length === 0 && state === 'offline') {
    body = <StudioEmptyState icon={Users} title="Servicio local sin conexión"
      description="Arranca el servicio local y vuelve a intentarlo para ver a los jugadores que sigues."
      actions={<Button variant="outline" onClick={() => void refresh()}>Reintentar</Button>} compact />;
  } else {
    const followedShown = shown !== null && followedIDs.has(shown.id);
    const place = tab === 'ranking' && rankingSelected !== null && choice !== null
      ? `Puesto ${rankingSelected.rank} en ${rankingChoiceLabel(options, choice)}` : undefined;
    body = <div className={WORKSPACE_GRID}>
      <PlayerRail tab={tab} onTabChange={setChosenTab} followedCount={players.length}
        seguidos={<PlayerRows players={players} label="Seguidos" selectedID={followedSelected?.id ?? null}
          onSelect={(id) => dispatch({ type: 'selected', id })}
          empty="Aún no sigues a nadie. Busca un nick o pega una URL de FACEIT arriba, o sigue a un jugador desde Ranking." />}
        ranking={<RankingPanel options={options} choice={choice} onChoose={chooseRanking} list={ranking} onMore={loadMoreRanking}
          onRetry={retryRanking} selectedID={rankingSelected?.id ?? null} onSelect={setRankingSelectedID} followedIDs={followedIDs} />} />
      {shown ? <section aria-label={`Perfil de ${shown.nickname}`} className="studio-panel min-w-0 overflow-hidden">
        <PlayerProfile player={shown} unfollowing={unfollowingID !== null} place={place}
          onUnfollow={followedShown ? () => void onUnfollow(shown.id) : undefined}
          onFollow={followedShown ? undefined : () => { if (!busy) void follow(shown.nickname, shown.id); }}
          followError={profileFollowFailure(followFailure, shown.id)} />
        <PlayerMatches key={shown.id} playerID={shown.id} enabled={state === 'ready'} />
      </section> : null}
    </div>;
  }

  const headerError = error ?? headerFollowFailure(followFailure);
  return (
    <div className="measure-work flex min-w-0 flex-col gap-8">
      <StudioPageHeader title="Jugadores FACEIT" description="Sigue jugadores de FACEIT, mira el ranking de un país, una región o una zona, revisa sus partidas y sube la demo de la que quieras convertir en clips."
        actions={<FollowPlayerForm query={query} onQueryChange={setQuery} onSubmit={(event) => void onFollow(event)}
          disabled={state !== 'ready'} busy={busy} />} />
      {/* The empty state already says offline with its own Reintentar; a second red line above it is noise. */}
      {headerError && !(state === 'offline' && players.length === 0) ? <div role="alert" className="flex flex-wrap items-center gap-3 text-body-sm text-destructive">
        <p>{headerError}</p>
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
  if (err.status === 409) return 'Ya sigues a 20 jugadores, el máximo. Deja de seguir a alguno para añadir a otro.';
  return err.message;
}
