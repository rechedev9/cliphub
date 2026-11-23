'use client';

import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { toast } from 'sonner';
import { LibraryApiError } from '@/lib/api/library';
import { extractDemoVoice, patchDemoVoiceLine, installTranscriber, transcribeDemoVoice } from '@/lib/api/voice';
import {
  SUBTITLES_LABEL, fetchJobSubtitles, subtitleAvailability, subtitleNote, subtitleRows, toggleHiddenPhrase,
  type JobSubtitles, type PhraseScope, type SubtitleChoice,
} from '@/lib/produce/subtitles';
import { voiceErrorText } from '@/lib/replay/voice-errors';
import { Button } from '@/components/ui/button';
import { PhraseRow, SubtitleToggle, TranscriberStatus } from './subtitle-parts';

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
  onAvailability?: (availability: string | null) => void;
  disabled?: boolean;
};

/** The Subtítulos panel of a producer: the switches, the installs they need, and the phrases of the chosen plays or rounds. */
export function SubtitlesPanel({ jobId, scope, voices, voicesLabel, planAvailability = null, value, onChange, onVoicesChange, onAvailability, disabled = false }: SubtitlesPanelProps): ReactNode {
  const [subs, setSubs] = useState<JobSubtitles | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [installError, setInstallError] = useState<string | null>(null);

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

  const checked = planAvailability !== null && planAvailability !== 'not_requested' ? planAvailability : subs?.availability ?? planAvailability;
  const availability = subtitleAvailability({ voices, voicesLabel, availability: checked });
  const reportAvailability = useRef(onAvailability);
  reportAvailability.current = onAvailability;
  useEffect(() => {
    reportAvailability.current?.(checked);
  }, [checked]);

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

  const install = useCallback(async (): Promise<void> => {
    setInstalling(true);
    setInstallError(null);
    try {
      await installTranscriber();
      await load();
    } catch (error) {
      setInstallError(voiceErrorText(error instanceof LibraryApiError ? error.code : undefined, 'No se pudo descargar el transcriptor.'));
    } finally {
      setInstalling(false);
    }
  }, [load]);

  const missing = value.enabled && availability.enabled && subs?.transcriber.state === 'not_installed';
  useEffect(() => {
    if (!missing || installing || installError !== null) return;
    void install();
  }, [install, installError, installing, missing]);

  const rows = subs === null ? [] : subtitleRows(subs.lines, scope, value);
  const note = subs === null || !value.enabled ? null : subtitleNote(subs);
  const locked = disabled || busy;
  const correct = (id: string, text: string, before: string): void => {
    const clean = text.trim();
    if (matchId === null || clean === '' || clean === before) return;
    void run(() => patchDemoVoiceLine({ matchId, lineId: id }, { text: clean }), 'No se pudo guardar la corrección.');
  };
  const discard = (id: string, discarded: boolean): void => {
    if (matchId === null) return;
    void run(() => patchDemoVoiceLine({ matchId, lineId: id }, { discarded }), discarded ? 'No se pudo descartar la frase.' : 'No se pudo recuperar la frase.');
  };

  return <div className="flex min-w-0 flex-col gap-3">
    <p className="text-body-sm text-fg-2">Cada frase de las voces del equipo, con el nombre de quien habla y un color fijo por jugador; el jugador que se ve sale en blanco. Si hablan varios a la vez, se ven como mucho 2 subtítulos en vertical y 3 en horizontal. Se transcriben en este PC una vez por demo. Una corrección vale para todos los vídeos de la demo y guarda lo que oyó el transcriptor; descarta las frases que nadie dijo.</p>
    {onVoicesChange ? <SubtitleToggle label={voicesLabel} checked={voices} disabled={disabled} onChange={onVoicesChange} /> : null}
    <SubtitleToggle label={SUBTITLES_LABEL} checked={value.enabled && availability.enabled} disabled={locked || !availability.enabled}
      onChange={(enabled) => onChange({ ...value, enabled })} />
    {availability.reason ? <p role="status" className="text-body-sm text-fg-2">{availability.reason}</p> : null}
    {value.enabled && availability.enabled ? <>
      {loadError ? <p role="alert" className="text-body-sm text-warning">{loadError}</p> : null}
      {subs ? <>
        <TranscriberStatus transcriber={subs.transcriber} busy={locked || installing} installError={installError} onInstall={() => void install()} />
        {note ? <p role="status" className="text-body-sm text-fg-2">{note}</p> : null}
        {matchId !== null && ['installed', 'installing'].includes(subs.transcriber.state) && (subs.state === 'none' || subs.state === 'failed') ? (
          <Button variant="secondary" size="sm" className="self-start" disabled={locked}
            onClick={() => void run(() => transcribeDemoVoice(matchId), 'No se pudo empezar la transcripción.')}>
            Transcribir ahora
          </Button>
        ) : null}
        {rows.length > 0 ? <ul className="flex flex-col gap-2" aria-label="Frases de este vídeo">
          {rows.map((row) => <PhraseRow key={row.id} row={row} locked={locked}
            heading={<><span className="font-semibold text-fg-1">{row.speaker || 'Jugador'}</span> · Ronda {row.round}</>}
            inputLabel={`Frase de ${row.speaker || 'un jugador'}`} discardedNote="No sale en ningún vídeo de esta demo."
            onHide={() => onChange(toggleHiddenPhrase(value, row.id))} onDiscard={(discarded) => discard(row.id, discarded)}
            onCorrect={(text) => correct(row.id, text, row.text)} />)}
        </ul> : null}
        {rows.length === 0 && subs.state === 'ready' ? <p className="text-body-sm text-fg-2">Nadie del equipo habla en lo que muestra este vídeo.</p> : null}
      </> : null}
    </> : null}
  </div>;
}
