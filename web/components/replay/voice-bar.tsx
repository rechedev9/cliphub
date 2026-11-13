'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { AudioLines, Check, Info, Mic, Radio, RotateCcw, Upload, Volume2, VolumeX, Wand2, X } from 'lucide-react';
import { MAX_VOICE_OFFSET_MS } from '@/lib/api/voice';
import type { MatchVoice } from '@/lib/api/voice';
import { currentEdition } from '@/lib/edition';
import { ADJUST_RANGE_MS, NUDGE_STEPS_MS, clampOffset, deltaLabel, formatVoiceMs } from '@/lib/replay/voice-sync';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { ReplayVoice } from './use-replay-voice';
import { VoicePluginMenu } from './voice-plugin-menu';

const AUDIO_ACCEPT = 'audio/*,.wav,.ogg,.opus,.mp3,.m4a,.flac';

const RANGE = cn(
  'h-6 cursor-pointer appearance-none bg-transparent outline-none',
  '[&::-webkit-slider-runnable-track]:h-1 [&::-webkit-slider-runnable-track]:rounded-full [&::-webkit-slider-runnable-track]:bg-surface-4',
  '[&::-webkit-slider-thumb]:-mt-1.5 [&::-webkit-slider-thumb]:size-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-fg-1',
  '[&::-moz-range-track]:h-1 [&::-moz-range-track]:bg-surface-4 [&::-moz-range-thumb]:size-4 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:bg-fg-1',
  FOCUS_RING,
);

function ImportButton({ voice, label }: { voice: ReplayVoice; label: string }): ReactNode {
  const input = useRef<HTMLInputElement>(null);
  return (
    <>
      <input
        ref={input}
        type="file"
        multiple
        accept={AUDIO_ACCEPT}
        className="hidden"
        onChange={(event) => {
          voice.importFiles(Array.from(event.target.files ?? []));
          event.target.value = '';
        }}
      />
      <Button type="button" size="xs" variant="outline" loading={voice.busy} onClick={() => input.current?.click()}>
        <Upload aria-hidden />
        {label}
      </Button>
    </>
  );
}

/** Where the recording is under the playhead, rewritten as the clock moves. */
function AudioPosition({ voice }: { voice: ReplayVoice }): ReactNode {
  const ref = useRef<HTMLSpanElement>(null);
  const { clock, demoMsAt, offsetMs } = voice;
  useEffect(() => {
    const write = (seconds: number): void => {
      const demoMs = demoMsAt(seconds);
      const text = demoMs === null ? 'sin ronda' : formatVoiceMs(demoMs - offsetMs);
      if (ref.current !== null && ref.current.textContent !== text) ref.current.textContent = text;
    };
    write(clock.seconds);
    return clock.subscribe(write);
  }, [clock, demoMsAt, offsetMs]);
  return <span ref={ref} className="w-[7ch] font-mono text-body-sm tabular-nums text-fg-1" />;
}

/** The slim offset bar of sync mode: drag or nudge the offset while the round plays, then confirm or cancel. */
function AdjustBar({ voice, saved }: { voice: ReplayVoice; saved: MatchVoice }): ReactNode {
  const [origin] = useState(saved.sync.offset_ms);
  const draft = voice.draft ?? origin;
  const set = (ms: number): void => voice.setDraft(clampOffset(ms, MAX_VOICE_OFFSET_MS));
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1.5 rounded-sm border border-border-subtle bg-surface-0 px-2 py-1" role="group" aria-label="Sincronizar la voz">
      <span className="flex items-center gap-1.5 font-mono text-body-sm tabular-nums text-fg-1" title="Desfase de la voz respecto a la demo">
        {formatVoiceMs(draft, true)}
        <Info className="size-3.5 text-fg-3" aria-hidden />
      </span>
      <Button type="button" size="xs" variant="ghost" onClick={() => set(saved.sync.auto_offset_ms)} title="Vuelve al desfase que calculó Studio">
        <RotateCcw aria-hidden />
        Restablecer
      </Button>
      <Button
        type="button"
        size="icon-xs"
        variant="default"
        aria-label="Confirmar la sincronización"
        loading={voice.busy}
        onClick={() => {
          void voice.saveOffset(draft).then((ok) => {
            if (ok) voice.setDraft(null);
          });
        }}
      >
        <Check aria-hidden />
      </Button>
      <Button type="button" size="icon-xs" variant="ghost" aria-label="Cancelar" onClick={() => voice.setDraft(null)}>
        <X aria-hidden />
      </Button>
      <input
        type="range"
        min={origin - ADJUST_RANGE_MS}
        max={origin + ADJUST_RANGE_MS}
        step={100}
        value={draft}
        onChange={(event) => set(Number(event.target.value))}
        aria-label="Desfase de la voz"
        aria-valuetext={formatVoiceMs(draft, true)}
        title="Arrastra para sincronizar"
        className={cn(RANGE, 'w-48 min-w-24 flex-1')}
      />
      <div className="flex items-center gap-0.5">
        {NUDGE_STEPS_MS.map((step) => (
          <Button key={step} type="button" size="xs" variant="ghost" className="tabular-nums" onClick={() => set(draft + step)}>
            {step > 0 ? '+' : '-'}
            {Math.abs(step) >= 1000 ? `${Math.abs(step) / 1000} s` : `0,${Math.abs(step) / 100} s`}
          </Button>
        ))}
      </div>
      <span className="flex items-baseline gap-1.5 text-meta text-fg-3">
        Audio
        <AudioPosition voice={voice} />
      </span>
      <span className="rounded-sm border border-border-strong bg-surface-2 px-1.5 font-mono text-body-sm tabular-nums text-fg-1" aria-live="polite">
        {deltaLabel(draft, origin)}
      </span>
    </div>
  );
}

function SyncState({ saved }: { saved: MatchVoice }): ReactNode {
  const { sync } = saved;
  if (sync.state === 'unsynced') {
    return (
      <span className="truncate text-meta font-medium text-warning" title={sync.note}>
        La voz aún no está sincronizada
      </span>
    );
  }
  return (
    <span className="truncate text-meta text-fg-3" title={sync.note}>
      {sync.state === 'manual' ? 'Sincronizada a mano' : 'Sincronizada sola'} · <span className="font-mono tabular-nums">{formatVoiceMs(sync.offset_ms, true)}</span>
    </span>
  );
}

/** Plays the demo's own in-game voice; the first press reads it from the demo file. */
function DemoVoiceToggle({ voice }: { voice: ReplayVoice }): ReactNode {
  const { demo, busy, on, toggle } = voice.demoVoice;
  if (demo === null || demo.state === 'unavailable') return null;
  if (demo.state === 'ready' && demo.tracks.length === 0) {
    return <span className="text-meta text-fg-3">La demo no trae voz</span>;
  }
  const extracting = demo.state === 'extracting';
  return (
    <Button
      type="button"
      size="xs"
      variant={on ? 'outline-primary' : 'ghost'}
      aria-pressed={on}
      loading={busy || extracting}
      onClick={toggle}
      title="La voz que los jugadores hablaron dentro del juego, si la demo la guardó"
    >
      <Radio aria-hidden />
      {extracting ? 'Leyendo la voz de la demo…' : 'Voz de la demo'}
    </Button>
  );
}

/** The web's voice row: the recordings play in Studio, so it only says where the voice comes from. */
function WebVoiceRow({ saved }: { saved: MatchVoice }): ReactNode {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <Mic className="size-3.5 shrink-0 text-fg-3" aria-hidden />
      <span className="truncate text-meta text-fg-3">
        {saved.present
          ? 'La voz del equipo se escucha en ClipHub Studio. Aquí se marca quién habla en cada momento.'
          : 'La voz del equipo se importa en ClipHub Studio y llega aquí al sincronizar.'}
      </span>
      {saved.present ? <div className="ml-auto"><SyncState saved={saved} /></div> : null}
    </div>
  );
}

/** Whether the voice of the round is being synced: the transport then swaps its round row for the offset bar. */
export function voiceSyncing(voice: ReplayVoice): boolean {
  return voice.draft !== null && voice.state.kind === 'ready' && voice.state.voice.present;
}

/** The offset bar of sync mode; nothing while the voice is not being synced. */
export function VoiceSyncBar({ voice }: { voice: ReplayVoice }): ReactNode {
  const { state } = voice;
  if (!voiceSyncing(voice) || state.kind !== 'ready') return null;
  return <AdjustBar key={state.voice.match_id} voice={voice} saved={state.voice} />;
}

/** The Replay's voice row between the round chips and the timeline: import, waveform, volume and mute, and the sync state. */
export function VoiceRow({ voice }: { voice: ReplayVoice }): ReactNode {
  const { state } = voice;
  if (state.kind === 'loading') return null;
  if (state.kind === 'unavailable') {
    return <p className="flex items-center gap-2 text-meta text-fg-3"><Mic className="size-3.5" aria-hidden />{state.reason}</p>;
  }
  const saved = state.voice;
  if (currentEdition() === 'web') return <WebVoiceRow saved={saved} />;
  if (!saved.present) {
    return (
      <div className="flex items-center gap-2">
        <Mic className="size-3.5 text-fg-3" aria-hidden />
        <span className="text-meta text-fg-3">Escucha la voz del equipo con la ronda: importa la grabación de TeamSpeak.</span>
        <ImportButton voice={voice} label="Importar voz" />
        <VoicePluginMenu voice={voice} />
        <div className="ml-auto">
          <DemoVoiceToggle voice={voice} />
        </div>
      </div>
    );
  }
  if (voice.draft !== null) return null;
  const unsynced = saved.sync.state === 'unsynced';
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1.5">
      <AudioLines className="size-4 shrink-0 text-fg-3" aria-hidden />
      <Button
        type="button"
        size="icon-xs"
        variant="ghost"
        aria-pressed={voice.muted}
        aria-label={voice.muted ? 'Activar la voz' : 'Silenciar la voz'}
        title="Voz del equipo"
        onClick={() => voice.setMuted(!voice.muted)}
      >
        {voice.muted ? <VolumeX aria-hidden /> : <Volume2 aria-hidden />}
      </Button>
      <input
        type="range"
        min={0}
        max={1}
        step={0.05}
        value={voice.volume}
        onChange={(event) => voice.setVolume(Number(event.target.value))}
        aria-label="Volumen de la voz"
        className={cn(RANGE, 'w-24')}
      />
      <div className="flex h-7 min-w-48 flex-1 items-center justify-center rounded-sm border border-border-subtle bg-surface-0 px-2">
        {unsynced ? (
          <button
            type="button"
            onClick={() => voice.setDraft(saved.sync.offset_ms)}
            title={saved.sync.note}
            className={cn('h-5 max-w-full truncate rounded-sm border border-destructive px-2 text-meta font-medium text-destructive outline-none hover:bg-destructive/10', FOCUS_RING)}
          >
            La voz no está sincronizada. Pulsa para sincronizarla.
          </button>
        ) : (
          <SyncState saved={saved} />
        )}
      </div>
      <div className="flex items-center gap-1">
        <DemoVoiceToggle voice={voice} />
        {unsynced ? null : (
          <Button type="button" size="xs" variant="outline" onClick={() => voice.setDraft(saved.sync.offset_ms)}>
            Sincronizar
          </Button>
        )}
        <Button type="button" size="xs" variant="ghost" loading={voice.busy} onClick={voice.autoSync} title="Studio vuelve a calcular cuándo empieza la grabación">
          <Wand2 aria-hidden />
          Calcular de nuevo
        </Button>
      </div>
    </div>
  );
}
