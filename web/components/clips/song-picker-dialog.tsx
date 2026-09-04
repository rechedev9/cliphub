'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Check, Pause, Play } from 'lucide-react';
import type { Song } from '@/lib/api/types';
import { api } from '@/lib/api';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export type SongPickerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called with the chosen song; the page stores it and closes the dialog. */
  onChoose: (songId: string, songTitle: string) => void;
  /** Currently selected song id (its row is marked as chosen). */
  selectedSongId?: string | null;
};

export type SongCatalogProps = {
  /** When false, playback stops and the catalog does not fetch. */
  active: boolean;
  selectedSongId?: string | null;
  onChoose: (song: Song) => void;
};

function mmss(totalSec: number): string {
  if (!totalSec || totalSec < 0) return '';
  const m = Math.floor(totalSec / 60);
  const s = Math.floor(totalSec % 60);
  return `${m}:${String(s).padStart(2, '0')}`;
}

/**
 * The curated soundtrack list plus one shared preview player. Used by the
 * pre-capture picker and by the Library music rerender dialog.
 */
export function SongCatalog({ active, selectedSongId, onChoose }: SongCatalogProps): ReactNode {
  const [songs, setSongs] = useState<Song[] | null>(null);
  const [playingId, setPlayingId] = useState<string | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const playRequestRef = useRef(0);

  useEffect(() => {
    if (!active) return;
    let mounted = true;
    (async () => {
      const next = await api.listSongs();
      if (mounted) setSongs(next);
    })();
    return () => {
      mounted = false;
    };
  }, [active]);

  useEffect(() => {
    if (active) return;
    audioRef.current?.pause();
    setPlayingId(null);
  }, [active]);

  async function togglePlay(song: Song): Promise<void> {
    const audio = audioRef.current;
    if (!audio) return;
    const request = ++playRequestRef.current;
    if (playingId === song.id) {
      audio.pause();
      setPlayingId(null);
      return;
    }
    if (!audio.paused) {
      await new Promise<void>((resolve) => {
        audio.addEventListener('pause', () => resolve(), { once: true });
        audio.pause();
      });
      if (request !== playRequestRef.current) return;
    }
    audio.src = song.previewUrl;
    audio.currentTime = 0;
    try {
      await audio.play();
      if (request === playRequestRef.current) setPlayingId(song.id);
    } catch {
      if (request === playRequestRef.current) setPlayingId(null);
    }
  }

  let list: ReactNode;
  if (songs === null) {
    list = <SongRowSkeletons />;
  } else if (songs.length === 0) {
    list = (
      <p className="px-2 py-8 text-center text-body-sm text-fg-2">
        No hay temas disponibles. Añade audio al directorio de música y recarga.
      </p>
    );
  } else {
    list = (
      <ul className="flex flex-col gap-1.5">
        {songs.map((song) => (
          <SongRow
            key={song.id}
            song={song}
            playing={playingId === song.id}
            selected={selectedSongId === song.id}
            onTogglePlay={() => void togglePlay(song)}
            onUse={() => onChoose(song)}
          />
        ))}
      </ul>
    );
  }

  return (
    <>
      <audio
        ref={audioRef}
        preload="none"
        onPause={() => setPlayingId(null)}
        onEnded={() => setPlayingId(null)}
        className="hidden"
      />
      <div className="-mx-2 max-h-[22rem] overflow-y-auto px-2">{list}</div>
    </>
  );
}

/**
 * SongPickerDialog — the soundtrack picker. Lists the orchestrator's curated
 * open-source catalog, plays a real audio preview per row (one shared <audio>),
 * and commits the chosen track to the reel. Music is optional; the reel is
 * created from the page's main CTA.
 */
export function SongPickerDialog({ open, onOpenChange, onChoose, selectedSongId }: SongPickerDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="font-[family-name:var(--font-display)] tracking-tight">ELIGE UN TEMA</DialogTitle>
          <DialogDescription>Cortamos la acción al ritmo del beat. Pulsa play para escucharlo.</DialogDescription>
        </DialogHeader>
        <SongCatalog
          active={open}
          selectedSongId={selectedSongId}
          onChoose={(song) => onChoose(song.id, song.title)}
        />
      </DialogContent>
    </Dialog>
  );
}

type SongRowProps = {
  song: Song;
  playing: boolean;
  selected: boolean;
  onTogglePlay: () => void;
  onUse: () => void;
};

function SongRow({ song, playing, selected, onTogglePlay, onUse }: SongRowProps) {
  const meta = [song.genre, mmss(song.durationSec)].filter(Boolean).join(' · ');
  return (
    <li
      className={cn(
        'flex items-center gap-3 rounded-lg border bg-card px-3 py-2.5',
        selected ? 'border-primary/60 ring-1 ring-primary/40' : 'border-border',
      )}
    >
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        onClick={onTogglePlay}
        aria-label={playing ? `Pausar ${song.title}` : `Escuchar ${song.title}`}
        className="shrink-0 text-muted-foreground hover:text-foreground"
      >
        {playing ? <Pause /> : <Play />}
      </Button>

      <div className="min-w-0 flex-1">
        <p className="truncate text-body-sm font-semibold text-fg-1">{song.title}</p>
        <p className="truncate font-mono text-meta tracking-wider text-fg-3">
          {song.artist}
          {/* No dimmer step below --fg-3 exists for text; the separator does the work. */}
          {meta ? <span> · {meta}</span> : null}
        </p>
      </div>

      {playing ? <Equalizer /> : null}

      <Button type="button" size="sm" variant={selected ? 'secondary' : 'default'} onClick={onUse} className="shrink-0">
        {selected ? <Check /> : null}
        {selected ? 'Elegido' : 'Usar este tema'}
      </Button>
    </li>
  );
}

/**
 * A decorative equalizer that animates while a preview plays. Keyframes are
 * scoped here (globals.css is foundation-owned) and honor prefers-reduced-motion.
 */
function Equalizer() {
  return (
    <span aria-hidden className="flex h-4 items-end gap-0.5">
      <style>{EQ_KEYFRAMES}</style>
      {[0, 1, 2, 3].map((i) => (
        <span
          key={i}
          className="ff-equalizer-bar w-0.5 rounded-full bg-primary"
          style={{ height: `${36 + (i % 3) * 18}%`, animationDelay: `${i * 0.12}s` }}
        />
      ))}
    </span>
  );
}

const EQ_KEYFRAMES = `
@keyframes ff-eq {
  0%, 100% { height: 30%; }
  50% { height: 100%; }
}
@media (prefers-reduced-motion: reduce) {
  @keyframes ff-eq { 0%, 100% { height: 55%; } }
}
`;

function SongRowSkeletons() {
  return (
    <ul className="flex flex-col gap-1.5">
      {[0, 1, 2, 3].map((i) => (
        <li key={i} className="flex items-center gap-3 rounded-lg border border-border bg-card px-3 py-2.5">
          <div className="size-8 shrink-0 animate-pulse rounded-md bg-accent" />
          <div className="flex-1 space-y-1.5">
            <div className="h-3.5 w-24 animate-pulse rounded bg-accent" />
            <div className="h-3 w-16 animate-pulse rounded bg-accent" />
          </div>
          <div className="h-8 w-28 animate-pulse rounded-md bg-accent" />
        </li>
      ))}
    </ul>
  );
}
