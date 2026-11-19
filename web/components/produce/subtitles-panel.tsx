'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { LibraryApiError } from '@/lib/api/library';
import { correctDemoVoiceLine, extractDemoVoice, installTranscriber, transcribeDemoVoice, type VoiceTranscriber } from '@/lib/api/voice';
import {
  SUBTITLES_LABEL, fetchJobSubtitles, subtitleAvailability, subtitleNote, subtitleRows, toggleHiddenPhrase,
  type JobSubtitles, type PhraseScope, type SubtitleChoice,
} from '@/lib/produce/subtitles';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

/** How often a transcription, an install or the reading of the demo voices is read again. */
const POLL_MS = 3000;

export type SubtitlesPanelProps = {
  jobId: string;
  scope: PhraseScope;
  /** The video carries the team voices; voicesLabel names the switch that adds them. */
  voices: boolean;
  voicesLabel: string;
  /** A Full Demo plan's own voice check, which wins over the library demo's. */
  planAvailability?: string | null;
  value: SubtitleChoice;
  onChange: (next: SubtitleChoice) => void;
  /** A Short turns its team voices on here; other videos have their own switch. */
  onVoicesChange?: (voices: boolean) => void;
  disabled?: boolean;
};

function Toggle({ label, checked, disabled, onChange }: { label: string; checked: boolean; disabled: boolean; onChange: (checked: boolean) => void }): ReactNode {
  return <label className={cn('flex min-h-10 items-center gap-3 text-body-sm text-fg-1', disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer')}>
    <input type="checkbox" className="size-4" checked={checked} disabled={disabled} onChange={(event) => onChange(event.target.checked)} />{label}
  </label>;
}

function megabytes(bytes: number): number {
  return Math.max(1, Math.round(bytes / (1 << 20)));
}

function InstallPrompt({ what, credit, install, busy, onInstall }: { what: string; credit: string; install: VoiceTranscriber; busy: boolean; onInstall: () => void }): ReactNode {
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

/** The Subtítulos panel of a producer: the switches, the installs they need, and the phrases of the chosen plays or rounds. */
export function SubtitlesPanel({ jobId, scope, voices, voicesLabel, planAvailability = null, value, onChange, onVoicesChange, disabled = false }: SubtitlesPanelProps): ReactNode {
  const [subs, setSubs] = useState<JobSubtitles | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (): Promise<void> => {
    try {
      setSubs(await fetchJobSubtitles(jobId));
      setLoadError(null);
    } catch {
      setLoadError('No se pudieron leer las voces de esta demo.');
    }
  }, [jobId]);

  useEffect(() => {
    void load();
  }, [load]);

  const waiting = subs !== null && (['queued', 'running'].includes(subs.state)
    || subs.transcriber.state === 'installing' || subs.availability === 'extracting');
  useEffect(() => {
    if (!waiting) return;
    const timer = window.setInterval(() => void load(), POLL_MS);
    return () => window.clearInterval(timer);
  }, [load, waiting]);

  // The demo voices are read once, so the switch can say whether the demo has them.
  const matchId = subs?.match_id ?? null;
  const unread = subs?.availability === 'not_requested';
  useEffect(() => {
    if (matchId === null || !unread) return;
    void extractDemoVoice(matchId).then(load, () => undefined);
  }, [load, matchId, unread]);

  const run = async (work: () => Promise<unknown>, fallback: string): Promise<void> => {
    setBusy(true);
    try {
      await work();
      await load();
    } catch (error) {
      toast.error(voiceErrorText(error instanceof LibraryApiError ? error.code : undefined, fallback));
    } finally {
      setBusy(false);
    }
  };

  // A plan made with the voices off has not checked them; the library demo may know.
  const checked = planAvailability !== null && planAvailability !== 'not_requested' ? planAvailability : subs?.availability ?? planAvailability;
  const availability = subtitleAvailability({ voices, voicesLabel, availability: checked });
  const rows = subs === null ? [] : subtitleRows(subs.lines, scope, value);
  const note = subs === null || !value.enabled ? null : subtitleNote(subs);
  const locked = disabled || busy;
  const correct = (id: string, text: string, before: string): void => {
    const clean = text.trim();
    if (matchId === null || clean === '' || clean === before) return;
    void run(() => correctDemoVoiceLine({ matchId, lineId: id }, { text: clean }), 'No se pudo guardar la corrección.');
  };

  return <div className="flex min-w-0 flex-col gap-3">
    <p className="text-body-sm text-fg-2">Cada frase de las voces del equipo, con el nombre de quien habla. Se transcriben en este PC una vez por demo.</p>
    {onVoicesChange ? <Toggle label={voicesLabel} checked={voices} disabled={disabled} onChange={onVoicesChange} /> : null}
    <Toggle label={SUBTITLES_LABEL} checked={value.enabled && availability.enabled} disabled={locked || !availability.enabled}
      onChange={(enabled) => onChange({ ...value, enabled })} />
    {availability.reason ? <p role="status" className="text-body-sm text-fg-2">{availability.reason}</p> : null}
    {value.enabled && availability.enabled ? <>
      {loadError ? <p role="alert" className="text-body-sm text-warning">{loadError}</p> : null}
      {subs ? <>
        <InstallPrompt what="transcripción" credit="Usa el modelo NVIDIA Parakeet TDT 0.6B v3, de NVIDIA, con licencia CC BY 4.0." install={subs.transcriber} busy={locked} onInstall={() => void run(installTranscriber, 'No se pudo instalar la transcripción.')} />
        {note ? <p role="status" className="text-body-sm text-fg-2">{note}</p> : null}
        {matchId !== null && subs.transcriber.state === 'installed' && (subs.state === 'none' || subs.state === 'failed') ? (
          <Button variant="secondary" size="sm" className="self-start" disabled={locked}
            onClick={() => void run(() => transcribeDemoVoice(matchId), 'No se pudo empezar la transcripción.')}>
            Transcribir ahora
          </Button>
        ) : null}
        {rows.length > 0 ? <ul className="flex flex-col gap-2" aria-label="Frases de este vídeo">
          {rows.map((row) => <li key={row.id} className={cn('flex flex-col gap-1.5 rounded-lg bg-surface-3 p-2.5', row.hidden && 'opacity-60')}>
            <div className="flex flex-wrap items-center justify-between gap-2 text-meta text-fg-2">
              <span><span className="font-semibold text-fg-1">{row.speaker || 'Jugador'}</span> · Ronda {row.round}{row.corrected ? ' · corregida' : ''}</span>
              <Button variant="ghost" size="sm" disabled={locked} aria-pressed={row.hidden} onClick={() => onChange(toggleHiddenPhrase(value, row.id))}>
                {row.hidden ? 'Mostrar en este vídeo' : 'Ocultar en este vídeo'}
              </Button>
            </div>
            <Input key={`${row.id}:${row.text}`} defaultValue={row.text} disabled={locked || row.hidden} maxLength={2000}
              aria-label={`Frase de ${row.speaker || 'un jugador'}`}
              onBlur={(event) => correct(row.id, event.target.value, row.text)} />
          </li>)}
        </ul> : null}
        {rows.length === 0 && subs.state === 'ready' ? <p className="text-body-sm text-fg-2">Nadie del equipo habla en lo que muestra este vídeo.</p> : null}
      </> : null}
    </> : null}
  </div>;
}
