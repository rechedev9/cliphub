'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { streamsApi, type StreamClipRange, type StreamPlay, type StreamPlayScan, type StreamVoiceSettings } from '@/lib/api/streams';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { isVoiceOnly, playIsCovered, playMoment, rankPlays } from '@/lib/streams/plays';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Segmented } from '@/components/ui/segmented';
import { Switch } from '@/components/ui/switch';
import { StreamSummaryCard } from '@/components/streams/stream-summary';

const POLL_MS = 2000;
const DEFAULT_VOICE: StreamVoiceSettings = { enabled: true, sensitivity: 3 };
const PRESETS = [
  { value: '1', label: 'Baja' },
  { value: '3', label: 'Media' },
  { value: '5', label: 'Alta' },
];

/** The presets, plus the level itself when the assistant set a 2 or a 4. */
function sensitivityOptions(sensitivity: number): { value: string; label: string }[] {
  if (PRESETS.some((preset) => preset.value === String(sensitivity))) return PRESETS;
  return [...PRESETS, { value: String(sensitivity), label: `Nivel ${sensitivity}` }].sort((a, b) => Number(a.value) - Number(b.value));
}

function voiceNote(scan: StreamPlayScan): string | null {
  switch (scan.voice?.status) {
    case 'no_audio':
      return 'El vídeo no tiene audio, así que no hemos buscado picos de voz.';
    case 'no_voice':
      return 'No hemos oído voz en el audio, así que no hay picos de voz.';
    case 'failed':
      return 'No pudimos leer el audio para buscar picos de voz.';
    case 'done':
      return scan.voice.mix ? 'Sin pista de micro: se usa la mezcla. La detección de picos de voz es menos precisa.' : null;
    default:
      return null;
  }
}

function VoiceSettingsRow({
  value,
  disabled,
  onChange,
}: {
  value: StreamVoiceSettings;
  disabled: boolean;
  onChange: (next: StreamVoiceSettings) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between gap-2">
        <span id="voice-peaks-label" className="text-body-sm text-fg-2">
          Picos de voz: marca cuando subes la voz por encima de lo normal
        </span>
        <Switch
          tone="stream"
          checked={value.enabled}
          disabled={disabled}
          aria-labelledby="voice-peaks-label"
          onCheckedChange={(enabled) => onChange({ ...value, enabled })}
        />
      </div>
      {value.enabled ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-body-sm text-fg-3">Sensibilidad</span>
          <Segmented
            label="Sensibilidad de los picos de voz"
            value={String(value.sensitivity)}
            options={sensitivityOptions(value.sensitivity)}
            onChange={(next) => onChange({ ...value, sensitivity: Number(next) })}
          />
        </div>
      ) : null}
    </div>
  );
}

function PlayRow({
  play,
  covered,
  disabled,
  onAdd,
  onSeek,
}: {
  play: StreamPlay;
  covered: boolean;
  disabled: boolean;
  onAdd: (plays: StreamPlay[]) => void;
  onSeek: (seconds: number) => void;
}): ReactNode {
  return (
    <li className="flex items-center justify-between gap-2 py-2">
      <button type="button" className="min-h-11 flex-1 rounded text-left text-body-sm hover:text-fg-1" onClick={() => onSeek(play.start_seconds)}>
        <span className="font-semibold">{play.label}</span>
        <span className="text-fg-3 tabular-nums"> · {formatStreamClock(playMoment(play))}</span>
        {play.marked ? <span className="text-fg-3"> · Marcado a propósito</span> : null}
        {!isVoiceOnly(play) && play.voice_seconds?.length ? <span className="text-fg-3"> · con voz alzada</span> : null}
      </button>
      {covered ? (
        <span className="text-meta text-fg-3">Añadida</span>
      ) : (
        <Button variant="ghost" size="sm" disabled={disabled} onClick={() => onAdd([play])}>
          Añadir
        </Button>
      )}
    </li>
  );
}

/**
 * Finds the streamer's kills in the source (the red kill-feed highlight of
 * CS2) and, from the mic, the moments the streamer raises their voice. Each
 * one is offered as a moment. A full VOD scans on its own and also offers a
 * landscape summary of the plays the user picks.
 */
export function StreamDetectedPlays({
  jobId,
  vod,
  clips,
  disabled,
  onAdd,
  onSeek,
}: {
  jobId: string;
  vod: boolean;
  clips: readonly StreamClipRange[];
  disabled: boolean;
  onAdd: (plays: StreamPlay[]) => void;
  onSeek: (seconds: number) => void;
}): ReactNode {
  const [scan, setScan] = useState<StreamPlayScan | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [voice, setVoice] = useState<StreamVoiceSettings>(DEFAULT_VOICE);
  const voiceSynced = useRef(false);
  const running = scan?.status === 'running';
  const autoStarted = useRef(false);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const next = await streamsApi.getPlays(jobId);
        if (!active) return;
        setScan(next);
        if (next.voice && !voiceSynced.current) {
          voiceSynced.current = true;
          setVoice({ enabled: next.voice.enabled, sensitivity: next.voice.sensitivity });
        }
      } catch (err) {
        if (active) setRequestError(errorMessage(err, 'No pudimos buscar jugadas. Vuelve a intentarlo.'));
      }
    };
    void load();
    const timer = running ? setInterval(() => void load(), POLL_MS) : undefined;
    return () => {
      active = false;
      if (timer) clearInterval(timer);
    };
  }, [jobId, running]);

  const start = async () => {
    setRequestError(null);
    try {
      voiceSynced.current = true;
      setScan(await streamsApi.startPlayScan(jobId, voice));
    } catch (err) {
      setRequestError(errorMessage(err, 'No pudimos buscar jugadas. Vuelve a intentarlo.'));
    }
  };

  useEffect(() => {
    if (!vod || disabled || scan?.status !== 'idle' || autoStarted.current) return;
    autoStarted.current = true;
    void start();
  });

  const ranked = rankPlays(scan?.plays ?? []);
  const killPlays = ranked.filter((play) => !isVoiceOnly(play));
  const voicePlays = ranked.filter(isVoiceOnly);
  const open = killPlays.filter((play) => !playIsCovered(clips, play));
  const scanned = scan?.voice;
  // A scan saved before voice peaks has no settings, so it can always be redone with them.
  const voiceChanged = scanned === undefined || scanned.enabled !== voice.enabled || (voice.enabled && scanned.sensitivity !== voice.sensitivity);
  const note = scan ? voiceNote(scan) : null;

  return (
    <section aria-label="Jugadas" className="flex flex-col gap-3 rounded-md border border-border-subtle p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold">Jugadas</h3>
        {scan?.status === 'done' && open.length > 1 ? (
          <Button variant="outline" size="sm" disabled={disabled} onClick={() => onAdd(open)}>
            Añadir todas
          </Button>
        ) : null}
      </div>

      {scan === null && !requestError ? <p className="text-body-sm text-fg-3">Comprobando…</p> : null}

      {scan?.status === 'idle' && !requestError ? (
        <>
          <p className="text-body-sm text-fg-2">
            Busca tus kills en el kill feed de CS2 y los momentos en que subes la voz, y crea un momento por cada jugada.
          </p>
          <VoiceSettingsRow value={voice} disabled={disabled} onChange={setVoice} />
          <Button variant="outline" disabled={disabled} onClick={() => void start()}>
            Buscar jugadas
          </Button>
        </>
      ) : null}

      {scan?.status === 'done' ? (
        <>
          <VoiceSettingsRow value={voice} disabled={disabled} onChange={setVoice} />
          {voiceChanged ? (
            <Button variant="outline" size="sm" disabled={disabled} onClick={() => void start()}>
              Buscar con estos ajustes
            </Button>
          ) : null}
          {note ? <p className="text-body-sm text-fg-3">{note}</p> : null}
        </>
      ) : null}

      {running ? (
        <div className="flex flex-col gap-2">
          <p className="text-body-sm text-fg-2" aria-live="polite">
            Buscando jugadas · {Math.round((scan?.progress ?? 0) * 100)} %
          </p>
          <Progress tone="stream" size="sm" value={(scan?.progress ?? 0) * 100} />
        </div>
      ) : null}

      {scan?.status === 'done' && ranked.length === 0 ? (
        <p className="text-body-sm text-fg-2">
          No hemos detectado kills en la kill feed (debe verse arriba a la derecha) ni picos de voz. Puedes marcar tus momentos a mano abajo.
        </p>
      ) : null}

      {vod && scan?.status === 'done' && ranked.length > 0 ? (
        <StreamSummaryCard jobId={jobId} plays={scan.plays} disabled={disabled} />
      ) : null}

      {scan?.status === 'done' && killPlays.length > 0 ? (
        <ul className="flex flex-col divide-y divide-border-subtle">
          {killPlays.map((play) => (
            <PlayRow key={play.id} play={play} covered={playIsCovered(clips, play)} disabled={disabled} onAdd={onAdd} onSeek={onSeek} />
          ))}
        </ul>
      ) : null}

      {scan?.status === 'done' && voicePlays.length > 0 ? (
        <div className="flex flex-col gap-1">
          <h4 className="text-body-sm font-semibold text-fg-2">Picos de voz</h4>
          <p className="text-body-sm text-fg-3">Momentos en que subiste la voz sin una kill cerca. No entran en Añadir todas.</p>
          <ul className="flex flex-col divide-y divide-border-subtle">
            {voicePlays.map((play) => (
              <PlayRow key={play.id} play={play} covered={playIsCovered(clips, play)} disabled={disabled} onAdd={onAdd} onSeek={onSeek} />
            ))}
          </ul>
        </div>
      ) : null}

      {scan?.status === 'failed' || requestError ? (
        <div className="flex flex-col gap-2">
          <p role="alert" className="text-body-sm text-destructive">
            {requestError ?? scan?.error ?? 'No pudimos buscar jugadas.'}
          </p>
          <Button variant="outline" disabled={disabled} onClick={() => void start()}>
            Volver a buscar
          </Button>
        </div>
      ) : null}
    </section>
  );
}
