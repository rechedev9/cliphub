'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Check, Mic, Radio, RotateCcw, Upload, Volume2, VolumeX, Wand2, X } from 'lucide-react';
import { MAX_VOICE_OFFSET_MS } from '@/lib/api/voice';
import type { MatchVoice } from '@/lib/api/voice';
import { ADJUST_RANGE_MS, NUDGE_STEPS_MS, clampOffset, deltaLabel, formatVoiceMs } from '@/lib/replay/voice-sync';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { ReplayVoice } from './use-replay-voice';

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

/** The EDGE-style adjust bar: drag or nudge the offset while the round plays, then confirm or cancel. */
function AdjustBar({ voice, saved }: { voice: ReplayVoice; saved: MatchVoice }): ReactNode {
  const [origin] = useState(saved.sync.offset_ms);
  const draft = voice.draft ?? origin;
  const set = (ms: number): void => voice.setDraft(clampOffset(ms, MAX_VOICE_OFFSET_MS));
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1.5" role="group" aria-label="Sincronizar la voz">
      <span className="flex items-baseline gap-1.5 text-meta text-fg-3">
        Audio
        <AudioPosition voice={voice} />
      </span>
      <input
        type="range"
        min={origin - ADJUST_RANGE_MS}
        max={origin + ADJUST_RANGE_MS}
        step={100}
        value={draft}
        onChange={(event) => set(Number(event.target.value))}
        aria-label="Desfase de la voz"
        aria-valuetext={formatVoiceMs(draft, true)}
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
      <span className="font-mono text-body-sm tabular-nums text-fg-1" aria-live="polite">
        {deltaLabel(draft, origin)}
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

/** The Replay's voice row: import, volume and mute, the sync state and the adjust bar. */
export function VoiceBar({ voice }: { voice: ReplayVoice }): ReactNode {
  const { state } = voice;
  if (state.kind === 'loading') return null;
  if (state.kind === 'unavailable') {
    return <p className="flex items-center gap-2 text-meta text-fg-3"><Mic className="size-3.5" aria-hidden />{state.reason}</p>;
  }
  const saved = state.voice;
  if (!saved.present) {
    return (
      <div className="flex items-center gap-2">
        <Mic className="size-3.5 text-fg-3" aria-hidden />
        <span className="text-meta text-fg-3">Escucha la voz del equipo con la ronda: importa la grabación de TeamSpeak.</span>
        <ImportButton voice={voice} label="Importar voz" />
        <div className="ml-auto">
          <DemoVoiceToggle voice={voice} />
        </div>
      </div>
    );
  }
  if (voice.draft !== null) return <AdjustBar key={saved.match_id} voice={voice} saved={saved} />;
  return (
    <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1.5">
      <Button
        type="button"
        size="icon-xs"
        variant="ghost"
        aria-pressed={voice.muted}
        aria-label={voice.muted ? 'Activar la voz' : 'Silenciar la voz'}
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
      <span className="text-meta text-fg-2">Voz del equipo</span>
      <SyncState saved={saved} />
      <div className="ml-auto flex items-center gap-1">
        <DemoVoiceToggle voice={voice} />
        <Button type="button" size="xs" variant={saved.sync.state === 'unsynced' ? 'outline-primary' : 'outline'} onClick={() => voice.setDraft(saved.sync.offset_ms)}>
          Sincronizar
        </Button>
        <Button type="button" size="xs" variant="ghost" loading={voice.busy} onClick={voice.autoSync} title="Studio vuelve a calcular cuándo empieza la grabación">
          <Wand2 aria-hidden />
          Calcular de nuevo
        </Button>
      </div>
    </div>
  );
}
