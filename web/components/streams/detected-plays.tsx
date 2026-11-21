'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Check, Crosshair, Mic } from 'lucide-react';
import { streamsApi, type StreamClipRange, type StreamPlay, type StreamPlayScan, type StreamVoiceSettings } from '@/lib/api/streams';
import { bulkAddLabel, bulkAddPlays, playRowAction, playTimeLabel, topPlayIds } from '@/lib/streams/moments';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { isVoiceOnly, orderPlays, playMoment, rankPlays, type PlayOrder } from '@/lib/streams/plays';
import { cn } from '@/lib/utils';
import { useSeen } from '@/components/streams/use-seen';
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
const ORDER_OPTIONS: readonly { value: PlayOrder; label: string }[] = [
  { value: 'best', label: 'Mejores primero' },
  { value: 'time', label: 'Por tiempo' },
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

function PlayThumb({ jobId, play }: { jobId: string; play: StreamPlay }): ReactNode {
  const { ref, seen } = useSeen<HTMLSpanElement>();
  const Icon = isVoiceOnly(play) ? Mic : Crosshair;
  return (
    <span ref={ref} className="relative block aspect-video w-20 shrink-0 overflow-hidden rounded bg-surface-3">
      {seen ? (
        <video
          src={`${streamsApi.sourceUrl(jobId)}#t=${playMoment(play)}`}
          preload="metadata"
          muted
          playsInline
          aria-hidden
          className="size-full object-cover"
        />
      ) : null}
      <span aria-hidden className="absolute bottom-1 left-1 grid size-5 place-items-center rounded-sm bg-surface-0/75 text-fg-1">
        <Icon className="size-3" />
      </span>
    </span>
  );
}

function PlayRow({
  jobId,
  play,
  clips,
  top,
  active,
  disabled,
  onAdd,
  onWatch,
  onJump,
}: {
  jobId: string;
  play: StreamPlay;
  clips: readonly StreamClipRange[];
  top: boolean;
  active: boolean;
  disabled: boolean;
  onAdd: (plays: StreamPlay[]) => void;
  onWatch: (play: StreamPlay) => void;
  onJump: (clip: StreamClipRange) => void;
}): ReactNode {
  const action = playRowAction(clips, play);
  const loud = !isVoiceOnly(play) && (play.voice_seconds?.length ?? 0) > 0;
  return (
    <li className={cn('flex items-center gap-2 px-1 py-1.5', active && 'bg-surface-3')}>
      <button
        type="button"
        aria-pressed={active}
        aria-label={`Ver jugada: ${play.label}, ${formatStreamClock(play.start_seconds)}`}
        className="flex min-h-11 min-w-0 flex-1 items-center gap-3 rounded text-left hover:text-fg-1"
        onClick={() => onWatch(play)}
      >
        <PlayThumb jobId={jobId} play={play} />
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="flex min-w-0 items-center gap-1.5 text-body-sm font-semibold">
            <span className="truncate">{play.label}</span>
            {top ? (
              <span className="shrink-0 rounded-sm bg-warning/20 px-1.5 text-meta font-bold uppercase text-warning">Top</span>
            ) : null}
          </span>
          <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-meta tabular-nums text-fg-3">
            {playTimeLabel(play)}
            {loud ? (
              <span className="inline-flex items-center gap-1">
                · <Mic aria-hidden className="size-3" /> con voz
              </span>
            ) : null}
            {play.marked ? <span>· Marcado a propósito</span> : null}
          </span>
        </span>
      </button>
      {action.kind === 'jump' ? (
        <Button variant="ghost" size="sm" disabled={disabled} className="shrink-0 text-success" onClick={() => onJump(action.clip)}>
          <Check aria-hidden />
          En tus momentos
        </Button>
      ) : (
        <Button variant="outline" size="sm" disabled={disabled} className="shrink-0" onClick={() => onAdd([play])}>
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
  activePlayId,
  onAdd,
  onWatch,
  onJump,
}: {
  jobId: string;
  vod: boolean;
  clips: readonly StreamClipRange[];
  disabled: boolean;
  activePlayId: string | null;
  onAdd: (plays: StreamPlay[]) => void;
  onWatch: (play: StreamPlay) => void;
  onJump: (clip: StreamClipRange) => void;
}): ReactNode {
  const [scan, setScan] = useState<StreamPlayScan | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [voice, setVoice] = useState<StreamVoiceSettings>(DEFAULT_VOICE);
  const [order, setOrder] = useState<PlayOrder>('best');
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

  const ranked = orderPlays(scan?.plays ?? [], order);
  const tops = topPlayIds(ranked);
  const open = bulkAddPlays(clips, rankPlays(ranked));
  const hasKills = ranked.some((play) => !isVoiceOnly(play));
  const scanned = scan?.voice;
  // A scan saved before voice peaks has no settings, so it can always be redone with them.
  const voiceChanged = scanned === undefined || scanned.enabled !== voice.enabled || (voice.enabled && scanned.sensitivity !== voice.sensitivity);
  const note = scan ? voiceNote(scan) : null;

  return (
    <section aria-label="Jugadas" className="flex min-h-0 min-w-0 flex-col gap-3 rounded-md border border-border-subtle bg-surface-1 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold">
          Jugadas{scan?.status === 'done' && ranked.length > 0 ? <span className="ml-1.5 text-fg-3 tabular-nums">{ranked.length}</span> : null}
        </h3>
        {scan?.status === 'done' && hasKills ? (
          <Button variant="stream" size="sm" disabled={disabled || open.length === 0} onClick={() => onAdd(open)}>
            {bulkAddLabel(open.length)}
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
          <p className="text-body-sm text-fg-3">Mientras tanto puedes ver el vídeo y marcar momentos a mano.</p>
        </div>
      ) : null}

      {scan?.status === 'done' && ranked.length === 0 ? (
        <p className="text-body-sm text-fg-2">
          No hemos detectado kills en la kill feed (debe verse arriba a la derecha) ni picos de voz. Puedes marcar tus momentos a mano con «Nuevo momento».
        </p>
      ) : null}

      {vod && scan?.status === 'done' && ranked.length > 0 ? (
        <StreamSummaryCard jobId={jobId} plays={scan.plays} disabled={disabled} />
      ) : null}

      {scan?.status === 'done' && ranked.length > 1 ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-body-sm text-fg-3">Orden</span>
          <Segmented label="Orden de las jugadas" value={order} options={ORDER_OPTIONS} onChange={setOrder} />
        </div>
      ) : null}

      {scan?.status === 'done' && ranked.length > 0 ? (
        <ul aria-label="Lista de jugadas" className="flex min-h-0 flex-col divide-y divide-border-subtle overflow-y-auto">
          {ranked.map((play) => (
            <PlayRow
              key={play.id}
              jobId={jobId}
              play={play}
              clips={clips}
              top={tops.has(play.id)}
              active={play.id === activePlayId}
              disabled={disabled}
              onAdd={onAdd}
              onWatch={onWatch}
              onJump={onJump}
            />
          ))}
        </ul>
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
