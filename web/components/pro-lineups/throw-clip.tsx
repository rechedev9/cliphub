'use client';

import { useState, type ReactNode } from 'react';
import { proClipUrl, roundClock, shownClip, type ProLineup } from '@/lib/api/pro-lineups';
import { formatShortDate } from '@/lib/format';

/** A recorded pro clip, looping and muted; says so plainly when Studio cannot load it. */
export function ClipVideo({ clip, label }: { clip: string; label: string }): ReactNode {
  const [failed, setFailed] = useState<string | null>(null);
  if (failed === clip) {
    return (
      <div className="flex aspect-video w-full items-center justify-center rounded-lg border border-border bg-surface-1 px-6 text-center">
        <p className="text-body-sm text-fg-2">No se pudo cargar el vídeo. Studio lo guarda la primera vez que lo ves con conexión.</p>
      </div>
    );
  }
  return (
    <video
      key={clip}
      src={proClipUrl(clip)}
      className="aspect-video w-full rounded-lg bg-black"
      autoPlay
      loop
      muted
      playsInline
      controls
      preload="auto"
      aria-label={label}
      onError={() => setFailed(clip)}
    />
  );
}

/** The recorded pro throw of a lineup, looping with the thrower's keys drawn on it; nothing until one is recorded. */
export function ThrowClip({ lineup, chosen }: { lineup: ProLineup; chosen: string | null }): ReactNode {
  const shown = shownClip(lineup, chosen);
  if (shown === null) return null;
  const { clip, example } = shown;
  return (
    <figure className="flex flex-col gap-1.5">
      <ClipVideo clip={clip} label={`Lanzamiento de ${example.player} en la ronda ${example.round}`} />
      <figcaption className="text-meta text-fg-2">
        {example.player}
        {example.team ? ` (${example.team})` : ''} · {example.event ? `${example.event} · ` : ''}R{example.round} · {roundClock(example.clock)} ·{' '}
        {formatShortDate(example.played_at)}. Las teclas que pulsa salen abajo, MOUSE1 incluido.
      </figcaption>
    </figure>
  );
}
