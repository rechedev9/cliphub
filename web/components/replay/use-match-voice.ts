'use client';

import { useCallback, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { LibraryApiError } from '@/lib/api/library';
import { attachVoiceRecording } from '@/lib/api/voice-plugin';
import {
  autoSyncVoice,
  deleteMatchVoice,
  getMatchVoice,
  getVoiceComms,
  importMatchVoice,
  installTranscriber,
  patchVoiceTrack,
  setVoiceOffset,
  transcribeMatchVoice,
} from '@/lib/api/voice';
import type { MatchVoice, VoiceComms } from '@/lib/api/voice';
import { voiceErrorText } from '@/lib/replay/voice-errors';

/** How often a running transcription or install is read again. */
const POLL_MS = 3000;

export type MatchVoiceState =
  | { kind: 'loading' }
  | { kind: 'unavailable'; reason: string }
  | { kind: 'ready'; voice: MatchVoice; comms: VoiceComms | null };

export type MatchVoiceControl = {
  state: MatchVoiceState;
  busy: boolean;
  importFiles: (files: readonly File[]) => void;
  saveOffset: (offsetMs: number) => Promise<boolean>;
  autoSync: () => void;
  nameTrack: (track: number, patch: { label?: string; steamid64?: string }) => void;
  remove: () => void;
  install: () => void;
  transcribe: (language: string) => void;
  /** Gives this match a recording the TeamSpeak plugin left in Studio's inbox. */
  attachRecording: (recordingId: string) => void;
};

/** The Spanish text of a failed request: its error code's, else the action's own. */
function failure(error: unknown, fallback: string): string {
  return voiceErrorText(error instanceof LibraryApiError ? error.code : undefined, fallback);
}

/** A library match's team voice and its comms, with what the voice bar and the comms panel change. */
export function useMatchVoice(matchId: string | null): MatchVoiceControl {
  const [state, setState] = useState<MatchVoiceState>({ kind: 'loading' });
  const [busy, setBusy] = useState(false);

  const load = useCallback(async (id: string): Promise<void> => {
    try {
      const voice = await getMatchVoice(id);
      const comms = voice.present ? await getVoiceComms(id).catch(() => null) : null;
      setState({ kind: 'ready', voice, comms });
    } catch (error) {
      setState({ kind: 'unavailable', reason: failure(error, 'No se pudo leer la voz de esta partida.') });
    }
  }, []);

  useEffect(() => {
    setState({ kind: 'loading' });
    if (matchId !== null) void load(matchId);
  }, [load, matchId]);

  const voice = state.kind === 'ready' ? state.voice : null;
  const waiting = voice !== null
    && (voice.transcript.state === 'queued' || voice.transcript.state === 'running' || voice.transcriber.state === 'installing');
  useEffect(() => {
    if (!waiting || matchId === null) return;
    const timer = window.setInterval(() => void load(matchId), POLL_MS);
    return () => window.clearInterval(timer);
  }, [load, matchId, waiting]);

  const run = useCallback(
    async (work: (id: string) => Promise<unknown>, fallback: string): Promise<boolean> => {
      if (matchId === null) return false;
      setBusy(true);
      try {
        await work(matchId);
        await load(matchId);
        return true;
      } catch (error) {
        toast.error(failure(error, fallback));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [load, matchId],
  );

  return {
    state,
    busy,
    importFiles: (files) => {
      if (files.length === 0) return;
      void run(async (id) => {
        const result = await importMatchVoice(id, files);
        const refused = result.files.filter((file) => file.status !== 'imported');
        for (const file of refused) toast.error(`${file.file_name}: ${voiceErrorText(file.error_code, 'no se pudo importar.')}`);
        if (result.sync.state === 'unsynced') toast.warning('La voz aún no está sincronizada: ajústala con Sincronizar.');
      }, 'No se pudo importar la voz.');
    },
    saveOffset: (offsetMs) => run((id) => setVoiceOffset(id, offsetMs), 'No se pudo guardar la sincronización.'),
    autoSync: () => void run((id) => autoSyncVoice(id), 'No se pudo sincronizar la voz.'),
    nameTrack: (track, patch) => void run((id) => patchVoiceTrack({ matchId: id, track }, patch), 'No se pudo cambiar la pista.'),
    remove: () => void run((id) => deleteMatchVoice(id), 'No se pudo quitar la voz.'),
    install: () => void run(() => installTranscriber(), 'No se pudo instalar la transcripción.'),
    transcribe: (language) => void run((id) => transcribeMatchVoice(id, language), 'No se pudo empezar la transcripción.'),
    attachRecording: (recordingId) => void run((id) => attachVoiceRecording({ id: recordingId, matchId: id }), 'No se pudo asociar la grabación.'),
  };
}
