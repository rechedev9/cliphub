'use client';

import { use, useEffect, useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { SearchX, Unplug } from 'lucide-react';
import { api } from '@/lib/api';
import type { Match, Play } from '@/lib/api/types';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { canForgeReel, reelCreativeBrief, type MusicBrief } from '@/lib/reel-brief';
import { FULL_DEMO_EDIT, FULL_DEMO_HREF, FULL_DEMO_VARIANT } from '@/lib/full-demo';
import { Button } from '@/components/ui/button';
import { StudioBackLink } from '@/components/studio/back-link';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { CreateReelBar } from '@/components/clips/create-reel-bar';
import { SongPickerDialog } from '@/components/clips/song-picker-dialog';

export default function FullDemoJobPage({ params }: { params: Promise<{ id: string }> }): ReactNode {
  const { id } = use(params);
  const router = useRouter();
  const [match, setMatch] = useState<Match | null>(null);
  const [plays, setPlays] = useState<Play[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [offline, setOffline] = useState(false);
  const [musicDecided, setMusicDecided] = useState(false);
  const [songId, setSongId] = useState<string | null>(null);
  const [songTitle, setSongTitle] = useState<string | null>(null);
  const [songOpen, setSongOpen] = useState(false);
  const [briefApproved, setBriefApproved] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  useEffect(() => {
    setBriefApproved(false);
  }, [musicDecided, songId]);

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
        hasPreset: true,
        selectionCount: plays.length,
        musicDecided,
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
        mode: songId ? 'music' : 'clean',
        songId: songId ?? undefined,
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

  let musicBrief: MusicBrief = { status: 'pending' };
  if (musicDecided && songTitle) {
    musicBrief = { status: 'track', title: songTitle, volumePercent: 100, gameVolumePercent: 20 };
  } else if (musicDecided) {
    musicBrief = { status: 'none' };
  }
  const briefItems = reelCreativeBrief(
    FULL_DEMO_EDIT,
    { name: FULL_DEMO_VARIANT, label: 'Viral 60 clean', description: '', hudMode: 'gameplay' },
    musicBrief,
  );

  return (
    <div className="flex flex-col gap-8">
      <StudioBackLink href={FULL_DEMO_HREF}>FULL DEMO TO VIDEO</StudioBackLink>
      <StudioPageHeader
        title={match.map.toUpperCase()}
        description={`${match.player ? `${match.player} · ` : ''}Todas las rondas parseadas entran en el vídeo. El brief de Shorts está cerrado.`}
      />

      <div className="flex flex-wrap gap-3">
        <Button type="button" variant="secondary" disabled={creating} onClick={() => setSongOpen(true)}>
          ELEGIR MÚSICA
        </Button>
        <Button
          type="button"
          variant="ghost"
          disabled={creating}
          onClick={() => {
            setSongId(null);
            setSongTitle(null);
            setMusicDecided(true);
          }}
        >
          SIN MÚSICA
        </Button>
        {musicDecided ? (
          <p className="self-center text-body-sm text-fg-2">
            {songTitle ?? 'Sin música de fondo'}
          </p>
        ) : (
          <p className="self-center text-body-sm text-fg-3">Decide la música antes de forjar.</p>
        )}
      </div>

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

      <CreateReelBar
        selectionLabel={plays.length === 0 ? null : `${plays.length} ${plays.length === 1 ? 'ronda' : 'rondas'}`}
        presetLabel="Full demo to video"
        songTitle={songTitle}
        musicDecided={musicDecided}
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

      <SongPickerDialog
        open={songOpen}
        onOpenChange={setSongOpen}
        onChoose={(chosenId, chosenTitle) => {
          setSongId(chosenId);
          setSongTitle(chosenTitle);
          setMusicDecided(true);
          setSongOpen(false);
        }}
        selectedSongId={songId}
      />
    </div>
  );
}
