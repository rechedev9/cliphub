'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Clapperboard, Smartphone, Video } from 'lucide-react';
import { createPlayerJob, getLibraryMatch, isLibraryOffline, type LibraryMatch } from '@/lib/api/library';
import {
  ANALYSIS_VIDEO_ACTION,
  ANALYSIS_VIDEO_ACTIONS,
  actionBlocker,
  analysisVideoHref,
  defaultVideoPlayer,
  videoPlayers,
  type AnalysisVideoAction,
  type VideoSource,
} from '@/lib/library/analysis-video';
import { matchTitle, teamByKey, teamLabel } from '@/lib/library/partidas';
import { formatShortDate } from '@/lib/format';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const ACTION_ICON: Record<AnalysisVideoAction, typeof Video> = { pov: Video, review: Clapperboard, short: Smartphone };

export type AnalysisVideoDialogProps = {
  /** The rounds a finding names, by match. Each video comes from one match. */
  sources: readonly VideoSource[];
  onClose: () => void;
  /** Rounds checked when the dialog opens; every round of the first match by default. */
  selected?: readonly number[];
  /** The team whose players are offered first, such as a report's rival. */
  teamId?: string;
  /** The POV the dialog opens with when that player played the match. */
  steamId?: string;
  title?: string;
};

/**
 * From analysis to video. The coach picks the match, the rounds and the POV;
 * the video is made by the existing Crear flows (long video for a POV or a
 * review, the Short producer for a Short), whose result lands in Mis vídeos
 * and in the match's Vídeos tab.
 */
export function AnalysisVideoDialog({ sources, onClose, selected, teamId, steamId, title = 'Crear vídeo' }: AnalysisVideoDialogProps): ReactNode {
  const router = useRouter();
  const [sourceIndex, setSourceIndex] = useState(0);
  const source = sources[sourceIndex] ?? sources[0];
  const [match, setMatch] = useState<LibraryMatch | null | 'failed' | 'offline'>(null);
  const [checked, setChecked] = useState<ReadonlySet<number>>(() => new Set(selected ?? source?.rounds ?? []));
  const [player, setPlayer] = useState<string>('');
  const [busy, setBusy] = useState<AnalysisVideoAction | null>(null);
  const [error, setError] = useState<string | null>(null);
  const matchId = source?.matchId ?? '';

  useEffect(() => {
    if (matchId === '') return;
    let active = true;
    setMatch(null);
    getLibraryMatch(matchId)
      .then((next) => {
        if (!active) return;
        setMatch(next);
        setPlayer(defaultVideoPlayer(next, { teamId, steamId }) ?? '');
      })
      .catch((failure: unknown) => {
        if (active) setMatch(isLibraryOffline(failure) ? 'offline' : 'failed');
      });
    return () => {
      active = false;
    };
  }, [matchId, teamId, steamId]);

  function chooseSource(index: number): void {
    setSourceIndex(index);
    setChecked(new Set(sources[index]?.rounds ?? []));
    setError(null);
  }

  function toggle(round: number): void {
    setChecked((current) => {
      const next = new Set(current);
      if (next.has(round)) next.delete(round);
      else next.add(round);
      return next;
    });
  }

  async function run(action: AnalysisVideoAction): Promise<void> {
    if (match === null || typeof match === 'string' || player === '') return;
    const rounds = [...checked].sort((a, b) => a - b);
    if (actionBlocker(action, rounds) !== null) return;
    setBusy(action);
    setError(null);
    try {
      const job = await createPlayerJob(match.demo_id, player);
      router.push(analysisVideoHref(action, job.job_id, rounds));
    } catch (failure) {
      setError(isLibraryOffline(failure) ? 'El servicio local no responde. Vuelve a intentarlo.' : 'No se pudo preparar el vídeo de esta partida.');
      setBusy(null);
    }
  }

  const rounds = [...checked].sort((a, b) => a - b);
  const loaded = match !== null && typeof match !== 'string' ? match : null;
  const players = loaded ? videoPlayers(loaded, teamId) : [];

  return (
    <Dialog open onOpenChange={(open) => !open && busy === null && onClose()}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            El vídeo se graba con el POV del jugador que elijas y se monta con los flujos de Crear. Lo encontrarás en Mis vídeos y en la ficha de la partida.
          </DialogDescription>
        </DialogHeader>

        {sources.length > 1 ? (
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1 text-meta text-fg-3">
              Estas rondas son de {sources.length} partidas. Cada vídeo sale de una partida.
            </legend>
            <div className="flex flex-wrap gap-1.5">
              {sources.map((s, index) => (
                <button
                  key={s.matchId}
                  type="button"
                  aria-pressed={index === sourceIndex}
                  onClick={() => chooseSource(index)}
                  className={cn(
                    'rounded-md border px-2.5 py-1 text-body-sm',
                    index === sourceIndex ? 'border-primary text-fg-1' : 'border-border text-fg-2 hover:text-fg-1',
                  )}
                >
                  Partida {index + 1} · {s.rounds.length} {s.rounds.length === 1 ? 'ronda' : 'rondas'}
                </button>
              ))}
            </div>
          </fieldset>
        ) : null}

        {loaded === null ? (
          <MatchPending state={match} />
        ) : (
          <div className="flex flex-col gap-4">
            <p className="text-body-sm text-fg-1">
              <span className="font-medium">{matchTitle(loaded)}</span>
              <span className="text-fg-3"> · {formatShortDate(loaded.played_at)}</span>
            </p>

            <fieldset className="flex flex-col gap-1.5">
              <legend className="mb-1 text-meta text-fg-3">Rondas</legend>
              <div className="flex flex-wrap gap-1.5">
                {(source?.rounds ?? []).map((round) => (
                  <label
                    key={round}
                    className={cn(
                      'flex cursor-pointer items-center gap-1.5 rounded-md border px-2 py-1 text-meta tabular-nums',
                      checked.has(round) ? 'border-primary text-fg-1' : 'border-border text-fg-2',
                    )}
                  >
                    <input type="checkbox" className="accent-[var(--primary)]" checked={checked.has(round)} onChange={() => toggle(round)} />
                    R{round}
                  </label>
                ))}
              </div>
            </fieldset>

            <div className="flex flex-col gap-1.5">
              <span id="analysis-video-player" className="text-meta text-fg-3">
                POV
              </span>
              {players.length === 0 ? (
                <p className="text-body-sm text-fg-2">Esta partida no tiene jugadores leídos.</p>
              ) : (
                <Select value={player} onValueChange={setPlayer}>
                  <SelectTrigger aria-labelledby="analysis-video-player" className="h-9">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {players.map((p) => (
                      <SelectItem key={p.steamid64} value={p.steamid64}>
                        {p.name} · {teamLabel(teamByKey(loaded, p.team_key))} · {p.kills} bajas
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>

            <ul className="flex flex-col gap-2">
              {ANALYSIS_VIDEO_ACTIONS.map((action) => {
                const meta = ANALYSIS_VIDEO_ACTION[action];
                const Icon = ACTION_ICON[action];
                const blocker = actionBlocker(action, rounds);
                return (
                  <li key={action} className="flex flex-wrap items-center justify-between gap-2 border border-border-subtle px-3 py-2">
                    <span className="flex min-w-0 flex-col">
                      <span className="text-body-sm font-medium text-fg-1">{meta.label}</span>
                      <span className="text-meta text-fg-3">{blocker ?? meta.hint}</span>
                    </span>
                    <Button
                      type="button"
                      size="sm"
                      variant={action === 'review' ? 'default' : 'outline'}
                      disabled={blocker !== null || player === '' || busy !== null}
                      loading={busy === action}
                      loadingText="Preparando"
                      onClick={() => void run(action)}
                    >
                      <Icon aria-hidden />
                      {meta.label}
                    </Button>
                  </li>
                );
              })}
            </ul>
            {error ? (
              <p role="alert" className="text-body-sm text-destructive">
                {error}
              </p>
            ) : null}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

function MatchPending({ state }: { state: null | 'failed' | 'offline' | LibraryMatch }): ReactNode {
  if (state === 'failed' || state === 'offline') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        {state === 'offline' ? 'El servicio local no responde.' : 'Esta partida ya no está en la biblioteca.'}
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-2" role="status" aria-label="Cargando la partida">
      <Skeleton className="h-5 w-64" />
      <Skeleton className="h-9 w-full" />
    </div>
  );
}
