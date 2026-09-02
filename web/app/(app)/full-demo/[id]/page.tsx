'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from '@/src/compat/link';
import { useParams, useRouter } from '@/src/compat/navigation';
import { AlertTriangle, Loader2, SearchX, Unplug } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { canForgeReel } from '@/lib/reel-brief';
import { startPollLoop } from '@/lib/poll-loop';
import { FullDemoCaptureBar } from '@/components/full-demo/capture-bar';
import { OVERLAY_THEME, type OverlayTheme } from '@/lib/api/types';
import {
  canStartFullDemoCapture,
  FULL_DEMO_EMPTY,
  FULL_DEMO_FORGE_HINT_EMPTY,
  FULL_DEMO_FORGE_HINT_ERROR,
  FULL_DEMO_HREF,
  FULL_DEMO_RECAP_ERROR,
  FULL_DEMO_ROUNDS_PENDING,
  FULL_DEMO_VARIANT,
  classifyFullDemoLoadFailure,
  fullDemoBriefItems,
  fullDemoEdit,
  fullDemoEmptyState,
  fullDemoOverlayThemeLabel,
  type FullDemoLoadFailure,
} from '@/lib/full-demo';
import { Button } from '@/components/ui/button';
import { StudioBackLink } from '@/components/studio/back-link';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';

const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

export default function FullDemoJobPage(): ReactNode {
  const { id = '' } = useParams();
  const router = useRouter();
  const [match, setMatch] = useState<Match | null>(null);
  const [plays, setPlays] = useState<Play[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadFailure, setLoadFailure] = useState<FullDemoLoadFailure>(null);
  const [recapFailure, setRecapFailure] = useState<Exclude<FullDemoLoadFailure, null> | null>(null);
  const [briefApproved, setBriefApproved] = useState(false);
  const [overlayTheme, setOverlayTheme] = useState<OverlayTheme>(OVERLAY_THEME.faceitOrange);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    const stop = startPollLoop({
      tick: async () => {
        try {
          const nextMatch = await api.getMatch(id);
          if (!active) return 'idle';
          setMatch(nextMatch);
          setLoadFailure(null);
          if (!nextMatch) {
            setPlays([]);
            setRecapFailure(null);
            setLoaded(true);
            return 'idle';
          }
          try {
            const nextPlays = await api.findRecapClips(id);
            if (!active) return 'idle';
            setPlays(nextPlays);
            setRecapFailure(null);
            setLoaded(true);
            return nextPlays.length === 0 ? 'fast' : 'idle';
          } catch (error) {
            if (!active) return 'idle';
            setPlays([]);
            setRecapFailure(classifyFullDemoLoadFailure(error));
            setLoaded(true);
            return 'idle';
          }
        } catch (error) {
          if (!active) return 'idle';
          setLoadFailure(classifyFullDemoLoadFailure(error));
          setMatch(null);
          setPlays([]);
          setRecapFailure(null);
          setLoaded(true);
          return 'idle';
        }
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });
    return () => {
      active = false;
      stop();
    };
  }, [id]);

  async function onCreate(): Promise<void> {
    if (
      !canForgeReel({
        briefApproved,
        creating,
        hasPreset: true,
        selectionCount: plays.length,
        musicDecided: true,
      }) ||
      !canStartFullDemoCapture({
        roundCount: plays.length,
        briefApproved,
        creating,
      })
    ) {
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      const video = await api.createVideo({
        matchId: id,
        playIds: plays.map((play) => play.id),
        mode: 'clean',
        variant: FULL_DEMO_VARIANT,
        editConfig: fullDemoEdit(overlayTheme),
      });
      router.push(`/videos?nuevo=${encodeURIComponent(video.id)}`);
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : 'No se pudo encolar la partida.');
      setCreating(false);
    }
  }

  if (!loaded) {
    return <p className="text-body-sm text-fg-2">Cargando la demo…</p>;
  }

  if (!match) {
    const empty = fullDemoEmptyState(loadFailure);
    let emptyIcon = SearchX;
    if (loadFailure === 'offline') emptyIcon = Unplug;
    else if (loadFailure === 'error') emptyIcon = AlertTriangle;
    return (
      <div className="flex flex-col gap-8">
        <StudioBackLink href={FULL_DEMO_HREF}>DEMO COMPLETA A VÍDEO</StudioBackLink>
        <StudioEmptyState
          icon={emptyIcon}
          title={empty.title}
          description={empty.description}
          actions={<Button onClick={() => router.push(FULL_DEMO_HREF)}>VOLVER</Button>}
        />
      </div>
    );
  }

  const briefItems = [
    ...fullDemoBriefItems(),
    { label: 'Tema overlays', value: fullDemoOverlayThemeLabel(overlayTheme) },
  ];
  const roundsPending = recapFailure === null && plays.length === 0;
  const povLabel = match.player
    ? `POV: ${match.player} · fijado al parsear · para otro jugador, vuelve a parsear la demo`
    : 'POV sin resolver — recarga o re-parsea';

  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <StudioBackLink href={FULL_DEMO_HREF}>DEMO COMPLETA A VÍDEO</StudioBackLink>
        <Link
          href={`/matches/${id}`}
          className="font-mono text-meta uppercase tracking-wider text-fg-3 transition-colors hover:text-primary"
        >
          Highlights 9:16 →
        </Link>
      </div>
      <StudioPageHeader
        title={match.map.toUpperCase()}
        description="Rondas en vivo desde el fin del freeze hasta la muerte del POV o el final de ronda."
      />

      <p className="font-mono text-body-sm text-fg-1" role="status">
        {povLabel}
      </p>

      {createError ? (
        <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
          {createError}
        </p>
      ) : null}

      {recapFailure === 'offline' ? (
        <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
          {FULL_DEMO_EMPTY.offline.title}. {FULL_DEMO_EMPTY.offline.description}
        </p>
      ) : null}
      {recapFailure === 'error' ? (
        <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
          {FULL_DEMO_RECAP_ERROR}
        </p>
      ) : null}
      {roundsPending ? (
        <p className="flex items-center gap-2 text-body-sm text-fg-2" role="status" aria-live="polite">
          <Loader2 className="size-4 shrink-0 animate-spin text-primary" aria-hidden />
          {FULL_DEMO_ROUNDS_PENDING}
        </p>
      ) : null}

      <FullDemoCaptureBar
        roundCount={plays.length}
        emptyHint={recapFailure ? FULL_DEMO_FORGE_HINT_ERROR : FULL_DEMO_FORGE_HINT_EMPTY}
        creating={creating}
        briefItems={[...briefItems]}
        briefApproved={briefApproved}
        onBriefApprovedChange={setBriefApproved}
        overlayTheme={overlayTheme}
        onOverlayThemeChange={setOverlayTheme}
        onCreate={() => {
          void onCreate();
        }}
      />
    </div>
  );
}
