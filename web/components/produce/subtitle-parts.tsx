'use client';

import type { ReactNode } from 'react';
import type { VoiceTranscriber } from '@/lib/api/voice';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

export const TRANSCRIBER_CREDIT = 'Usa el modelo NVIDIA Parakeet TDT 0.6B v3, de NVIDIA, con licencia CC BY 4.0.';

export function SubtitleToggle({ label, checked, disabled, onChange }: { label: string; checked: boolean; disabled: boolean; onChange: (checked: boolean) => void }): ReactNode {
  return <label className={cn('flex min-h-10 items-center gap-3 text-body-sm text-fg-1', disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer')}>
    <input type="checkbox" className="size-4" checked={checked} disabled={disabled} onChange={(event) => onChange(event.target.checked)} />{label}
  </label>;
}

function megabytes(bytes: number): number {
  return Math.max(1, Math.round(bytes / (1 << 20)));
}

export function InstallPrompt({ what, credit, install, busy, onInstall }: { what: string; credit: string; install: VoiceTranscriber; busy: boolean; onInstall: () => void }): ReactNode {
  if (install.state === 'installed') return null;
  if (install.state === 'unsupported') return <p className="text-body-sm text-warning">La {what} solo funciona en Windows de 64 bits.</p>;
  if (install.state === 'installing') {
    const done = install.download_bytes > 0 ? Math.round(((install.done_bytes ?? 0) / install.download_bytes) * 100) : 0;
    return <p role="status" className="text-body-sm text-fg-2">Instalando la {what}… {done} % de unos {megabytes(install.download_bytes)} MB</p>;
  }
  return <div className="flex flex-col gap-1.5">
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="secondary" size="sm" disabled={busy} onClick={onInstall}>Instalar la {what} (unos {megabytes(install.download_bytes)} MB)</Button>
      {install.state === 'failed' ? <span className="text-body-sm text-warning">{voiceErrorText(install.error_code, 'La instalación falló.')}</span> : null}
    </div>
    <p className="text-meta text-fg-3">Se descarga una vez y funciona sin conexión. {credit}</p>
  </div>;
}

export type PhraseRowData = { id: string; text: string; corrected: boolean; original: string | null; discarded: boolean; hidden: boolean };

export type PhraseRowProps = {
  row: PhraseRowData;
  heading: ReactNode;
  inputLabel: string;
  discardedNote: string;
  locked: boolean;
  onHide: (hidden: boolean) => void;
  onDiscard: (discarded: boolean) => void;
  onCorrect: (text: string) => void;
};

export function PhraseRow({ row, heading, inputLabel, discardedNote, locked, onHide, onDiscard, onCorrect }: PhraseRowProps): ReactNode {
  return <li className={cn('flex flex-col gap-1.5 rounded-lg bg-surface-3 p-2.5', (row.hidden || row.discarded) && 'opacity-60')}>
    <div className="flex flex-wrap items-center justify-between gap-2 text-meta text-fg-2">
      <span>{heading}{row.corrected ? ' · corregida' : ''}{row.discarded ? ' · descartada' : ''}</span>
      <span className="flex flex-wrap items-center gap-1">
        {row.discarded ? null : <Button variant="ghost" size="sm" disabled={locked} aria-pressed={row.hidden} onClick={() => onHide(!row.hidden)}>
          {row.hidden ? 'Mostrar en este vídeo' : 'Ocultar en este vídeo'}
        </Button>}
        <Button variant="ghost" size="sm" disabled={locked} aria-pressed={row.discarded} onClick={() => onDiscard(!row.discarded)}>
          {row.discarded ? 'Recuperar la frase' : 'Descartar en todos los vídeos'}
        </Button>
      </span>
    </div>
    <Input key={`${row.id}:${row.text}`} defaultValue={row.text} disabled={locked || row.hidden || row.discarded} maxLength={2000}
      aria-label={inputLabel} onBlur={(event) => onCorrect(event.target.value)} />
    {row.original === null ? null : <div className="flex flex-wrap items-center justify-between gap-2 text-meta text-fg-3">
      <span>El transcriptor oyó: «{row.original}»</span>
      <Button variant="ghost" size="sm" disabled={locked || row.discarded} onClick={() => onCorrect(row.original ?? '')}>Volver al original</Button>
    </div>}
    {row.discarded ? <p className="text-meta text-fg-3">{discardedNote}</p> : null}
  </li>;
}
