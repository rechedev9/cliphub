'use client';

import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Upload } from 'lucide-react';
import type { EditConfig } from '@/lib/api/types';
import { humanErrorText } from '@/lib/human-error';
import {
  PLAYER_CARD_SLOTS,
  fetchPlayerCardInfo,
  playerCardPreviewUrl,
  removePlayerCardImage,
  uploadPlayerCardImage,
  withPlayerCard,
  type PlayerCardImage,
  type PlayerCardInfo,
} from '@/lib/player-cards';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { Button } from '@/components/ui/button';

const NONE = 'ninguna';

const IMAGE_LABEL: Record<PlayerCardImage, string> = {
  upload: 'la imagen que subiste',
  faceit: 'su avatar de FACEIT',
  '': 'sus iniciales, porque no tiene imagen',
};

export type PlayerCardOptionsProps = {
  value: EditConfig;
  onChange: (next: EditConfig) => void;
  disabled?: boolean;
  jobId: string;
  /** Why this Short cannot carry a card, known before asking Studio (several players). */
  blockedReason?: string | undefined;
};

/** The opening, end and cover slots of a one-player Short, each "Ninguna" or a card template. */
export function PlayerCardOptions({ value, onChange, disabled = false, jobId, blockedReason }: PlayerCardOptionsProps): ReactNode {
  const [info, setInfo] = useState<PlayerCardInfo | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [version, setVersion] = useState(0);
  const [busy, setBusy] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  const id = useId();

  useEffect(() => {
    if (blockedReason) return undefined;
    const controller = new AbortController();
    fetchPlayerCardInfo(jobId, controller.signal).then(setInfo, (failure: unknown) => {
      if (!controller.signal.aborted) setMessage(humanErrorText({ error: failure, lead: 'No se pudieron leer las cartas de jugador.' }));
    });
    return () => controller.abort();
  }, [jobId, blockedReason, version]);

  async function changeImage(change: () => Promise<void>): Promise<void> {
    setBusy(true);
    setMessage(null);
    try {
      await change();
      setVersion((current) => current + 1);
    } catch (failure) {
      setMessage(humanErrorText({ error: failure, lead: 'No se pudo cambiar la imagen.' }));
    } finally {
      setBusy(false);
    }
  }

  const reason = blockedReason ?? (info && !info.available ? info.reason : '');
  const templates = info?.templates ?? [];
  const player = reason ? null : info?.player ?? null;
  const off = disabled || busy || reason !== '' || player === null;
  return (
    <div className="flex min-w-0 flex-col gap-3">
      {reason ? <p className="text-body-sm text-fg-3">{reason}</p> : null}
      {!reason && info === null && message === null ? <p role="status" className="text-body-sm text-fg-3">Cargando las cartas…</p> : null}
      {player ? (
        <ul className="flex min-w-0 gap-2 overflow-x-auto pb-1" aria-label="Vista previa de las cartas">
          {templates.map((template) => (
            <li key={template.id} className="flex w-24 shrink-0 flex-col gap-1">
              <img
                src={playerCardPreviewUrl(jobId, template.id, version)}
                alt={`Carta ${template.name} de ${player.name}`}
                loading="lazy"
                className="aspect-[9/16] w-full rounded border border-border-subtle bg-surface-3 object-cover"
              />
              <span className="text-meta text-fg-3">{template.name}</span>
            </li>
          ))}
        </ul>
      ) : null}
      {PLAYER_CARD_SLOTS.map((slot) => {
        const noCover = slot.key === 'coverCard' && value.coverStrategy === 'no-cover';
        return (
          <div key={slot.key} className="flex min-w-0 flex-col gap-1.5">
            <span id={`${id}-${slot.key}`} className="text-meta text-fg-3">{slot.label}</span>
            <ToggleGroup
              type="single"
              value={value[slot.key] ?? NONE}
              onValueChange={(next) => { if (next) onChange(withPlayerCard(value, slot.key, next === NONE ? null : next)); }}
              disabled={off || noCover}
              variant="outline"
              spacing={1.5}
              className="flex-wrap"
              aria-labelledby={`${id}-${slot.key}`}
            >
              <ToggleGroupItem value={NONE} aria-label={`${slot.label}: ninguna`}>Ninguna</ToggleGroupItem>
              {templates.map((template) => (
                <ToggleGroupItem key={template.id} value={template.id} title={template.description} aria-label={`${slot.label}: ${template.name}`}>
                  {template.name}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            {noCover ? <p className="text-body-sm text-fg-3">Elige una portada para poner la carta encima.</p> : null}
          </div>
        );
      })}
      {player ? (
        <div className="flex min-w-0 flex-col gap-2">
          <span id={`${id}-image`} className="text-meta text-fg-3">Imagen del jugador</span>
          <p className="text-body-sm text-fg-2">La carta de {player.name} muestra {IMAGE_LABEL[player.image]}.</p>
          <input ref={input} type="file" accept="image/png,image/jpeg" className="sr-only" tabIndex={-1} aria-labelledby={`${id}-image`} disabled={off}
            onChange={(event) => { const file = event.target.files?.[0]; event.target.value = ''; if (file) void changeImage(() => uploadPlayerCardImage(jobId, file)); }} />
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <Button type="button" variant="secondary" aria-describedby={`${id}-image`} disabled={off} onClick={() => input.current?.click()}>
              <Upload aria-hidden />{player.image === 'upload' ? 'Cambiar imagen' : 'Subir imagen'}
            </Button>
            {player.image === 'upload' ? (
              <Button type="button" variant="ghost" disabled={off} onClick={() => void changeImage(() => removePlayerCardImage(jobId))}>Quitar</Button>
            ) : null}
          </div>
          <p className="text-body-sm text-fg-3">PNG o JPG, hasta 10 MB. Se guarda para este jugador y sus próximas cartas la usan. Sin imagen, la carta usa su avatar de FACEIT o sus iniciales.</p>
        </div>
      ) : null}
      {message ? <p role="alert" className="text-body-sm text-destructive">{message}</p> : null}
    </div>
  );
}
