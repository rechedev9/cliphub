'use client';

import { use, useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { SearchX, Unplug } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { canForgeReel, reelCreativeBrief } from '@/lib/reel-brief';
import { FullDemoStylePicker } from '@/components/full-demo/style-picker';
import { FULL_DEMO_EDIT, FULL_DEMO_HREF, FULL_DEMO_PRESET, FULL_DEMO_VARIANT } from '@/lib/full-demo';
import { Button } from '@/components/ui/button';
import { StudioBackLink } from '@/components/studio/back-link';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { CreateReelBar } from '@/components/clips/create-reel-bar';

export default function FullDemoJobPage({ params }: { params: Promise<{ id: string }> }): ReactNode {
  const { id } = use(params);
  const router = useRouter();
  const [match, setMatch] = useState<Match | null>(null);
  const [plays, setPlays] = useState<Play[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [offline, setOffline] = useState(false);
  const [variant, setVariant] = useState<string | null>(null);
  const [briefApproved, setBriefApproved] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        const [nextMatch, nextPlays] = await Promise.all([api.getMatch(id), api.findRecapClips(id)]);
        if (!active) return;
        setMatch(nextMatch);
        setPlays(nextPlays);
        setOffline(false);
      } catch (error) {
        if (!active) return;
        setOffline((error as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE);
        setMatch(null);
      } finally {
        if (active) setLoaded(true);
      }
    })();
    return () => {
      active = false;
    };
  }, [id]);

  async function onCreate(): Promise<void> {
    if (
      !canForgeReel({
        briefApproved,
        creating,
        hasPreset: variant === FULL_DEMO_VARIANT,
        selectionCount: plays.length,
        musicDecided: true,
      })
    ) {
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      await api.createVideo({
        matchId: id,
        playIds: plays.map((play) => play.id),
        mode: 'clean',
        variant: FULL_DEMO_VARIANT,
        editConfig: FULL_DEMO_EDIT,
      });
      router.push('/videos');
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : 'No se pudo encolar la partida.');
      setCreating(false);
    }
  }

  if (!loaded) {
    return <p className="text-body-sm text-fg-2">Cargando la demo…</p>;
  }

  if (!match) {
    return (
      <div className="flex flex-col gap-8">
        <StudioBackLink href={FULL_DEMO_HREF}>FULL DEMO TO VIDEO</StudioBackLink>
        <StudioEmptyState
          icon={offline ? Unplug : SearchX}
          title={offline ? 'Servicio local sin conexión' : 'Demo no encontrada'}
          description={
            offline
              ? 'Arranca ClipHub y vuelve a abrir esta partida.'
              : 'Esta demo ya no está en el disco local.'
          }
          actions={<Button onClick={() => router.push(FULL_DEMO_HREF)}>VOLVER</Button>}
        />
      </div>
    );
  }

  const selectedPreset = variant === FULL_DEMO_VARIANT ? FULL_DEMO_PRESET : null;
  const briefItems = reelCreativeBrief(
    FULL_DEMO_EDIT,
    selectedPreset,
    { status: 'none' },
  );

  return (
    <div className="flex flex-col gap-8">
      <StudioBackLink href={FULL_DEMO_HREF}>FULL DEMO TO VIDEO</StudioBackLink>
      <StudioPageHeader
        title={match.map.toUpperCase()}
        description={`${match.player ? `${match.player} · ` : ''}Rondas en vivo (sin freeze) con HUD nativo y comms. Sin música. El brief de Shorts está cerrado.`}
      />

      {createError ? (
        <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
          {createError}
        </p>
      ) : null}

      {plays.length === 0 ? (
        <p className="text-body-sm text-fg-2">
          Esta demo no tiene plan de rondas todavía. Espera a que termine el parseo o elige otra.
        </p>
      ) : null}

      <FullDemoStylePicker
        value={variant}
        onChange={setVariant}
        disabled={plays.length === 0 || creating}
      />

      <CreateReelBar
        selectionLabel={plays.length === 0 ? null : `${plays.length} ${plays.length === 1 ? 'ronda' : 'rondas'}`}
        presetLabel={selectedPreset?.label ?? null}
        songTitle={null}
        musicDecided
        format={FULL_DEMO_EDIT.format}
        onFormatChange={() => undefined}
        formatLocked
        creating={creating}
        briefItems={briefItems}
        briefApproved={briefApproved}
        onBriefApprovedChange={setBriefApproved}
        onCreate={() => {
          void onCreate();
        }}
      />
    </div>
  );
}
