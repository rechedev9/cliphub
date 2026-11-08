'use client';

import { useState, type ReactNode } from 'react';
import { proClipUrl, roundClock, shownClip, type ProLineup } from '@/lib/api/pro-lineups';
import { formatShortDate } from '@/lib/format';

/** The recorded pro throw of a lineup, looping with the thrower's keys drawn on it. */
export function ThrowClip({ lineup, chosen }: { lineup: ProLineup; chosen: string | null }): ReactNode {
  const shown = shownClip(lineup, chosen);
  const [failed, setFailed] = useState<string | null>(null);
  if (shown === null) {
    return (
      <div className="flex aspect-video w-full flex-col items-center justify-center gap-1 rounded-lg border border-border bg-surface-1 px-6 text-center">
        <p className="text-body-sm font-medium text-fg-1">Aún no hay vídeo de este lineup</p>
        <p className="text-meta text-fg-2">Se graba cuando entra una partida pro nueva con este lanzamiento. Mientras, tienes el radar y la posición para la consola.</p>
      </div>
    );
  }
  const { clip, example } = shown;
  return (
    <figure className="flex flex-col gap-1.5">
      {failed === clip ? (
        <div className="flex aspect-video w-full items-center justify-center rounded-lg border border-border bg-surface-1 px-6 text-center">
          <p className="text-body-sm text-fg-2">No se pudo cargar el vídeo. Studio lo guarda la primera vez que lo ves con conexión.</p>
        </div>
      ) : (
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
          aria-label={`Lanzamiento de ${example.player} en la ronda ${example.round}`}
          onError={() => setFailed(clip)}
        />
      )}
      <figcaption className="text-meta text-fg-2">
        {example.player}
        {example.team ? ` (${example.team})` : ''} · {example.event ? `${example.event} · ` : ''}R{example.round} · {roundClock(example.clock)} ·{' '}
        {formatShortDate(example.played_at)}. Las teclas que pulsa salen abajo, MOUSE1 incluido.
      </figcaption>
    </figure>
  );
}
