'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { FULL_DEMO_VOICE_LABEL, demoHasNoVoices, fullDemoVoiceStatus } from '@/lib/produce/full-demo-brief';
import { FullDemoGain, FullDemoGroup, FullDemoToggle } from './full-demo-fields';

type Props = { options: FullDemoOptions; voiceAvailability: string | null; onChange: (options: FullDemoOptions) => void };

export function FullDemoAudio({ options, voiceAvailability, onChange }: Props): ReactNode {
  const { audio } = options;
  const { voice, game } = audio;
  const change = (patch: Partial<FullDemoOptions['audio']>): void => onChange({ ...options, audio: { ...audio, ...patch } });
  const status = fullDemoVoiceStatus(voiceAvailability);
  const noVoices = demoHasNoVoices(voiceAvailability);
  const availability = voice.enabled || status.problem ? status : null;
  return <FullDemoGroup title="Sonido" note="Audio de partida equilibrado automáticamente, sin música de fondo.">
    <FullDemoToggle label={FULL_DEMO_VOICE_LABEL} value={voice.enabled && !noVoices} disabled={noVoices} describedBy={noVoices ? 'full-demo-voice-status' : undefined}
      onChange={(enabled) => change({ voice: { ...voice, enabled } })} />
    {availability ? <p id="full-demo-voice-status" className={availability.problem ? 'text-body-sm text-warning' : 'text-body-sm text-fg-2'} role="status">{availability.text}</p> : null}
    <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
      <FullDemoGain label="Juego" value={game.gain} onChange={(gain) => change({ game: { ...game, gain } })} />
      <FullDemoGain label="Voces" value={voice.gain} disabled={!voice.enabled || noVoices} onChange={(gain) => change({ voice: { ...voice, gain } })} />
    </div>
  </FullDemoGroup>;
}
