'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Check, Crosshair, Mic, SlidersHorizontal } from 'lucide-react';
import { streamsApi, type StreamClipRange, type StreamPlay, type StreamPlayScan, type StreamVoiceSettings } from '@/lib/api/streams';
import { detectionNote, detectionSettingsChanged, shoutOptions } from '@/lib/streams/detection';
import { bulkAddLabel, bulkAddPlays, playRowAction, playTimeLabel, topPlayIds } from '@/lib/streams/moments';
import { errorMessage, formatStreamClock } from '@/lib/streams/plan';
import { playNumbers } from '@/lib/streams/timeline';
import { isVoiceOnly, orderPlays, playMoment, rankPlays, type PlayOrder } from '@/lib/streams/plays';
import { cn } from '@/lib/utils';
import { useSeen } from '@/components/streams/use-seen';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Segmented } from '@/components/ui/segmented';
import { Switch } from '@/components/ui/switch';

const POLL_MS = 2000;
const NO_PLAYS: readonly StreamPlay[] = [];
const DEFAULT_VOICE: StreamVoiceSettings = { enabled: true, sensitivity: 3 };
const ORDER_OPTIONS: readonly { value: PlayOrder; label: string }[] = [
  { value: 'best', label: 'Mejores primero' },
  { value: 'time', label: 'Por tiempo' },
];
const SETTINGS_ID = 'stream-detection-settings';

function DetectionSettings({
  value,
  note,
  disabled,
  onChange,
  children,
}: {
  value: StreamVoiceSettings;
  note: string | null;
  disabled: boolean;
  onChange: (next: StreamVoiceSettings) => void;
  children?: ReactNode;
}): ReactNode {
  return (
    <div id={SETTINGS_ID} className="flex flex-col gap-3 rounded-md border border-border-subtle bg-surface-2 p-3">
      <div className="flex items-center justify-between gap-3">
        <span className="flex flex-col gap-0.5">
          <span id="voice-peaks-label" className="text-body-sm font-semibold">
            Incluir picos de voz
          </span>
          <span className="text-meta text-fg-3">Momentos en los que gritas, aunque no haya kill.</span>
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
        <div className="flex flex-wrap items-center justify-between gap-3">
          <span className="flex flex-col gap-0.5">
            <span className="text-body-sm font-semibold">Cuánto tienes que gritar</span>
            <span className="text-meta text-fg-3">Con «Poco» salen más picos de voz.</span>
          </span>
          <Segmented
            label="Cuánto tienes que gritar"
            value={String(value.sensitivity)}
            options={shoutOptions(value.sensitivity)}
            onChange={(next) => onChange({ ...value, sensitivity: Number(next) })}
          />
        </div>
      ) : null}
      {note ? <p className="text-body-sm text-fg-3">{note}</p> : null}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-meta text-fg-3">Cambiar esto no toca los momentos que ya has elegido.</span>
        {children}
      </div>
    </div>
  );
}

function PlayThumb({ jobId, play, number }: { jobId: string; play: StreamPlay; number: number }): ReactNode {
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
      <span aria-hidden className="absolute bottom-1 left-1 flex h-5 items-center gap-0.5 rounded-sm bg-surface-0/75 px-1 text-meta font-semibold tabular-nums text-fg-1">
        <Icon className="size-3" />
        {number}
      </span>
    </span>
  );
}

function PlayRow({
  jobId,
  play,
  number,
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
  number: number;
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
  const rowRef = useRef<HTMLLIElement>(null);
  useEffect(() => {
    if (active) rowRef.current?.scrollIntoView({ block: 'nearest' });
  }, [active]);
  return (
    <li ref={rowRef} className={cn('flex items-center gap-2 px-1 py-1.5', active && 'bg-surface-3')}>
      <button
        type="button"
        aria-pressed={active}
        aria-label={`Ver jugada ${number}: ${play.label}, ${formatStreamClock(play.start_seconds)}`}
        className="flex min-h-11 min-w-0 flex-1 items-center gap-3 rounded text-left hover:text-fg-1"
        onClick={() => onWatch(play)}
      >
        <PlayThumb jobId={jobId} play={play} number={number} />
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
 * one is offered as a moment. A full VOD scans on its own.
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
  onPlaysShown,
}: {
  jobId: string;
  vod: boolean;
  clips: readonly StreamClipRange[];
  disabled: boolean;
  activePlayId: string | null;
  onAdd: (plays: StreamPlay[]) => void;
  onWatch: (play: StreamPlay) => void;
  onJump: (clip: StreamClipRange) => void;
  onPlaysShown: (plays: readonly StreamPlay[]) => void;
}): ReactNode {
  const [scan, setScan] = useState<StreamPlayScan | null>(null);
  const [requestError, setRequestError] = useState<string | null>(null);
  const [voice, setVoice] = useState<StreamVoiceSettings>(DEFAULT_VOICE);
  const [order, setOrder] = useState<PlayOrder>('best');
  const [settingsOpen, setSettingsOpen] = useState(false);
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

  const shownPlays = scan?.status === 'done' ? scan.plays : NO_PLAYS;
  useEffect(() => onPlaysShown(shownPlays), [shownPlays, onPlaysShown]);

  const ranked = orderPlays(scan?.plays ?? [], order);
  const numbers = playNumbers(ranked);
  const tops = topPlayIds(ranked);
  const open = bulkAddPlays(clips, rankPlays(ranked));
  const hasKills = ranked.some((play) => !isVoiceOnly(play));
  const voiceChanged = detectionSettingsChanged(scan?.voice, voice);
  const note = detectionNote(scan?.voice);
  const settingsButton = (
    <Button
      variant="ghost"
      size="sm"
      aria-expanded={settingsOpen}
      aria-controls={SETTINGS_ID}
      onClick={() => setSettingsOpen((open) => !open)}
    >
      <SlidersHorizontal aria-hidden />
      Ajustes de detección
    </Button>
  );

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
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" disabled={disabled} onClick={() => void start()}>
              Buscar jugadas
            </Button>
            {settingsButton}
          </div>
          {settingsOpen ? <DetectionSettings value={voice} note={null} disabled={disabled} onChange={setVoice} /> : null}
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

      {scan?.status === 'done' ? (
        <div className="flex flex-wrap items-center gap-2">
          {ranked.length > 1 ? (
            <>
              <span className="text-body-sm text-fg-3">Orden</span>
              <Segmented label="Orden de las jugadas" value={order} options={ORDER_OPTIONS} onChange={setOrder} />
            </>
          ) : null}
          <span className="ml-auto">{settingsButton}</span>
        </div>
      ) : null}

      {scan?.status === 'done' && settingsOpen ? (
        <DetectionSettings value={voice} note={note} disabled={disabled} onChange={setVoice}>
          {voiceChanged ? (
            <Button variant="outline" size="sm" disabled={disabled} onClick={() => void start()}>
              Volver a buscar
            </Button>
          ) : null}
        </DetectionSettings>
      ) : null}

      {scan?.status === 'done' && ranked.length > 0 ? (
        <ul aria-label="Lista de jugadas" className="flex min-h-0 flex-col divide-y divide-border-subtle overflow-y-auto">
          {ranked.map((play) => (
            <PlayRow
              key={play.id}
              jobId={jobId}
              play={play}
              number={numbers.get(play.id) ?? 0}
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
