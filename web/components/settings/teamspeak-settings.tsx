'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Mic } from 'lucide-react';
import { listLibraryMatches, LibraryApiError, type LibraryMatch } from '@/lib/api/library';
import {
  attachVoiceRecording,
  deleteVoiceRecording,
  getVoicePlugin,
  listVoiceRecordings,
  setupVoicePlugin,
  type VoicePluginStatus,
  type VoiceRecording,
} from '@/lib/api/voice-plugin';
import { getDesktopSettingsBridge, teamSpeakInstallError } from '@/lib/desktop-settings';
import { matchLabel, recordingMatchChoices, recordingTitle } from '@/lib/voice-plugin-view';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { SettingsSection } from './settings-section';

type PluginState =
  | { kind: 'loading' }
  | { kind: 'offline' }
  | { kind: 'ready'; status: VoicePluginStatus; recordings: VoiceRecording[]; matches: LibraryMatch[] };

type Notice = { tone: 'ok' | 'error'; text: string } | null;

/** Installs the TeamSpeak 3 plugin and CS2's game state config, and lists the recordings waiting for their match. */
export function TeamSpeakSettings(): ReactNode {
  const [state, setState] = useState<PluginState>({ kind: 'loading' });
  const [notice, setNotice] = useState<Notice>(null);
  const [installing, setInstalling] = useState(false);

  const load = useCallback(async (): Promise<void> => {
    try {
      const [status, list, matches] = await Promise.all([getVoicePlugin(), listVoiceRecordings(), listLibraryMatches()]);
      setState({ kind: 'ready', status, recordings: list.recordings, matches });
    } catch {
      setState({ kind: 'offline' });
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const install = async (): Promise<void> => {
    setInstalling(true);
    setNotice(null);
    try {
      const status = await setupVoicePlugin().catch((error: unknown) => {
        if (error instanceof LibraryApiError && error.code === 'cs2_not_found') return null;
        throw error;
      });
      const bridge = getDesktopSettingsBridge();
      if (bridge?.installTeamSpeakPlugin === undefined) {
        setNotice({ tone: 'error', text: 'El plugin se instala desde la app de escritorio de Studio.' });
        return;
      }
      const failure = teamSpeakInstallError(await bridge.installTeamSpeakPlugin());
      if (failure !== null) {
        setNotice({ tone: 'error', text: failure });
        return;
      }
      setNotice({
        tone: 'ok',
        text: status === null
          ? 'TeamSpeak te pedirá confirmar la instalación. No encontramos CS2: graba con el atajo del plugin.'
          : 'TeamSpeak te pedirá confirmar la instalación. Reinicia CS2 para que grabe solo en tus partidas.',
      });
    } catch {
      setNotice({ tone: 'error', text: 'No se pudo preparar la instalación. Inténtalo de nuevo.' });
    } finally {
      setInstalling(false);
      void load();
    }
  };

  let body: ReactNode;
  if (state.kind === 'loading') {
    body = <Skeleton role="status" aria-label="Leyendo el plugin de TeamSpeak" className="h-24 w-full rounded-none" />;
  } else if (state.kind === 'offline') {
    body = <p className="text-body-sm text-fg-2">Studio no responde ahora. Vuelve a abrir Ajustes en un momento.</p>;
  } else {
    const pending = state.recordings.filter((recording) => recording.status === 'pending');
    body = (
      <>
        <PluginStatusLines status={state.status} />
        {pending.length > 0 ? (
          <div className="flex flex-col gap-3">
            <h3 className="font-display text-body-sm font-bold text-fg-1">Grabaciones sin partida</h3>
            {pending.map((recording) => (
              <PendingRecording key={recording.id} recording={recording} matches={state.matches} onChange={load} />
            ))}
          </div>
        ) : null}
      </>
    );
  }

  return (
    <SettingsSection
      id="teamspeak"
      title="Voz del equipo en TeamSpeak"
      description="Un plugin de TeamSpeak 3 graba una pista por persona mientras juegas una partida de CS2, y Studio la coloca sola en su demo. Solo graba con la grabación activada en TeamSpeak, y el audio no sale de este PC."
      status={state.kind === 'ready' && state.status.game_state === 'installed' ? <Badge variant="success">Listo</Badge> : null}
    >
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" onClick={() => void install()} disabled={installing}>
          <Mic aria-hidden />
          Instalar plugin de TeamSpeak
        </Button>
      </div>
      {notice ? (
        <p role={notice.tone === 'error' ? 'alert' : 'status'} className={notice.tone === 'error' ? 'text-body-sm text-destructive' : 'text-body-sm text-fg-2'}>
          {notice.text}
        </p>
      ) : null}
      {body}
    </SettingsSection>
  );
}

function PluginStatusLines({ status }: { status: VoicePluginStatus }): ReactNode {
  let cs2: string;
  if (!status.cs2_found) cs2 = 'No encontramos CS2 en este PC: el plugin solo graba con su atajo.';
  else if (status.game_state === 'installed') cs2 = 'CS2 avisa a Studio cuando empieza una partida.';
  else if (status.game_state === 'stale') cs2 = 'La configuración de CS2 es de otro arranque: vuelve a pulsar Instalar.';
  else cs2 = 'Falta la configuración de CS2: pulsa Instalar.';
  const live = status.live.live
    ? `Partida en curso${status.live.map ? ` en ${status.live.map}` : ''}: el plugin graba si la grabación está activada.`
    : null;
  return (
    <ul className="flex flex-col gap-1 text-body-sm text-fg-2">
      <li>{cs2}</li>
      {live ? <li className="text-fg-1">{live}</li> : null}
      <li className="break-all text-meta text-fg-3">Grabaciones en {status.inbox}</li>
    </ul>
  );
}

type PendingProps = { recording: VoiceRecording; matches: LibraryMatch[]; onChange: () => Promise<void> };

function PendingRecording({ recording, matches, onChange }: PendingProps): ReactNode {
  const choices = recordingMatchChoices(recording, matches);
  const [matchId, setMatchId] = useState(choices[0]?.id ?? '');
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (action: () => Promise<unknown>, failure: string): Promise<void> => {
    setBusy(true);
    setError(null);
    try {
      await action();
      await onChange();
    } catch (cause) {
      setError(cause instanceof LibraryApiError && cause.code === 'voice_present' ? 'Esa partida ya tiene voz del equipo.' : failure);
    } finally {
      setBusy(false);
      setConfirming(false);
    }
  };

  return (
    <div className="flex flex-col gap-2 border border-border bg-surface-2 px-3.5 py-3">
      <p className="text-body-sm font-semibold text-fg-1">{recordingTitle(recording)}</p>
      {recording.note ? <p className="text-meta text-fg-2">{recording.note}</p> : null}
      <p className="text-meta text-fg-3">{recording.tracks.map((track) => track.nickname).join(', ')}</p>
      <div className="flex flex-wrap items-center gap-2">
        {choices.length > 0 ? (
          <>
            <Select value={matchId} onValueChange={setMatchId}>
              <SelectTrigger aria-label="Partida de la grabación" className="h-8 min-w-0 flex-1 text-meta">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {choices.map((match) => (
                  <SelectItem key={match.id} value={match.id}>{matchLabel(match)}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              size="sm"
              disabled={busy || matchId === ''}
              onClick={() => void run(() => attachVoiceRecording({ id: recording.id, matchId }), 'No se pudo asociar la grabación.')}
            >
              Asociar
            </Button>
          </>
        ) : (
          <p className="text-meta text-fg-2">Importa la demo de esta partida y Studio la asociará sola.</p>
        )}
        {confirming ? (
          <>
            <Button type="button" size="sm" variant="destructive" disabled={busy} onClick={() => void run(() => deleteVoiceRecording(recording.id), 'No se pudo borrar la grabación.')}>
              Borrar de este PC
            </Button>
            <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={() => setConfirming(false)}>
              Cancelar
            </Button>
          </>
        ) : (
          <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => setConfirming(true)}>
            Borrar
          </Button>
        )}
      </div>
      {error ? <p role="alert" className="text-meta text-destructive">{error}</p> : null}
    </div>
  );
}
