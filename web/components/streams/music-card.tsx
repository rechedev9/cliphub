'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Loader2, Pause, Play } from 'lucide-react';
import type { Song } from '@/lib/api/types';
import { MUSIC_VOLUMES, NO_MUSIC_VALUE } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { StreamStepCard, StreamSwitch } from '@/components/streams/step-card';

/**
 * Background music and the viral grade. The music bed is mixed under the
 * streamer's own audio at render time, so the gain presets here are the same
 * three the plan stores — the slider-free choice is deliberate.
 */
export function StreamMusicCard({
  songs,
  musicKey,
  volume,
  grade,
  busy,
  onMusicKey,
  onMusicVolume,
  onGrade,
}: {
  /** Catalog owned by the editor (the rail needs titles before this step opens); null while loading. */
  songs: Song[] | null;
  musicKey: string;
  volume: number;
  grade: boolean;
  busy: boolean;
  onMusicKey: (key: string) => void;
  onMusicVolume: (volume: number) => void;
  onGrade: (grade: boolean) => void;
}): ReactNode {
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
  const previewRequest = useRef(0);

  const selectedSong = songs?.find((song) => song.id === musicKey);

  const stopAndResetPreview = useCallback(() => {
    previewRequest.current += 1;
    const audio = audioRef.current;
    if (audio) {
      audio.pause();
      audio.currentTime = 0;
    }
    setPreviewPlaying(false);
  }, []);

  useEffect(() => {
    stopAndResetPreview();
    setPreviewError(null);
  }, [musicKey, busy, stopAndResetPreview]);

  useEffect(() => {
    const audio = audioRef.current;
    return () => {
      previewRequest.current += 1;
      if (audio) {
        audio.pause();
        audio.currentTime = 0;
      }
    };
  }, []);

  const togglePreview = async (): Promise<void> => {
    const audio = audioRef.current;
    if (!audio || !selectedSong?.previewUrl || busy || songs === null) return;

    if (previewPlaying) {
      previewRequest.current += 1;
      audio.pause();
      setPreviewPlaying(false);
      return;
    }

    const request = ++previewRequest.current;
    setPreviewError(null);
    try {
      await audio.play();
      if (previewRequest.current === request) setPreviewPlaying(true);
    } catch {
      if (previewRequest.current !== request) return;
      audio.pause();
      audio.currentTime = 0;
      setPreviewPlaying(false);
      setPreviewError('No se pudo reproducir la vista previa de esta canción.');
    }
  };

  let selectedMusicLabel = 'Ninguna';
  if (songs === null) {
    selectedMusicLabel = musicKey ? 'Cargando pista…' : 'Cargando pistas…';
  } else if (selectedSong) {
    selectedMusicLabel = `${selectedSong.title}${selectedSong.genre ? ` · ${selectedSong.genre}` : ''}`;
  }

  return (
    <div className="flex flex-col gap-2.5">
      <StreamStepCard title="Pista del catálogo">
        <div className="flex items-center gap-2">
          <Select
            value={musicKey || NO_MUSIC_VALUE}
            disabled={busy || songs === null}
            onValueChange={(value) => onMusicKey(value === NO_MUSIC_VALUE ? '' : value)}
          >
            <SelectTrigger id="stream-music" aria-label="Música de fondo" className="h-9 min-w-0 flex-1">
              <SelectValue>{selectedMusicLabel}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NO_MUSIC_VALUE}>Ninguna</SelectItem>
              {(songs ?? []).map((song) => (
                <SelectItem key={song.id} value={song.id}>
                  {song.title}
                  {song.genre ? ` · ${song.genre}` : ''}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            disabled={busy || songs === null || !selectedSong?.previewUrl}
            onClick={() => void togglePreview()}
            aria-label={`${previewPlaying ? 'Pausar' : 'Escuchar'} ${selectedSong?.title ?? 'música seleccionada'}`}
            className="size-9 shrink-0"
          >
            {previewPlaying ? <Pause aria-hidden /> : <Play aria-hidden />}
          </Button>
        </div>
        <audio
          ref={audioRef}
          src={selectedSong?.previewUrl}
          preload="none"
          data-music-preview
          className="hidden"
          onPlay={() => setPreviewPlaying(true)}
          onPause={() => setPreviewPlaying(false)}
          onEnded={stopAndResetPreview}
          onError={() => {
            stopAndResetPreview();
            setPreviewError('No se pudo reproducir la vista previa de esta canción.');
          }}
        />
        {songs === null ? (
          <p role="status" className="flex items-center gap-2 font-mono text-meta uppercase tracking-wider text-fg-3">
            <Loader2 aria-hidden className="size-3.5 animate-spin" />
            Cargando catálogo local
          </p>
        ) : null}
        {songs !== null && songs.length === 0 ? (
          <p className="text-body-sm text-fg-3">
            No hay pistas en el catálogo local. El render seguirá adelante sin música.
          </p>
        ) : null}
        {previewError ? (
          <p role="alert" className="text-body-sm text-destructive">
            {previewError}
          </p>
        ) : null}
        <p className="text-body-sm text-fg-3">La música se mezcla bajo el audio del streamer.</p>
      </StreamStepCard>

      {musicKey ? (
        <StreamStepCard title="Volumen de la música">
          <ToggleGroup
            type="single"
            variant="filter"
            spacing={2}
            value={String(volume)}
            onValueChange={(v) => v && onMusicVolume(Number(v))}
            disabled={busy}
            aria-label="Volumen de música"
            className="w-full [&>*]:flex-1"
          >
            {MUSIC_VOLUMES.map((v) => (
              <ToggleGroupItem key={v.value} value={String(v.value)} className="font-mono uppercase tracking-wider">
                {v.label} · {Math.round(v.value * 100)}%
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        </StreamStepCard>
      ) : null}

      <StreamStepCard
        title="Efecto grade"
        control={<StreamSwitch label="Efecto grade" checked={grade} disabled={busy} onChange={onGrade} />}
      >
        <p className="font-display text-body-sm font-semibold uppercase text-fg-1">
          {grade ? 'Contraste y saturación viral' : 'Desactivado'}
        </p>
        <p className="text-body-sm text-fg-3">Ligero realce de contraste y saturación en cada Short.</p>
      </StreamStepCard>
    </div>
  );
}
