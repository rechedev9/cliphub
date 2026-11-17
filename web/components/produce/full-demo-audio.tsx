'use client';

import type { ReactNode } from 'react';
import type { FullDemoDocument, FullDemoOptions } from '@/lib/full-demo-plan';
import { FULL_DEMO_VOICE_LABEL, fullDemoVoiceStatus } from '@/lib/produce/full-demo-brief';
import { FullDemoGain, FullDemoGroup, FullDemoToggle } from './full-demo-fields';

type Props = { options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void };

export function FullDemoAudio({ options, document, onChange }: Props): ReactNode {
  const { audio } = options;
  const { voice, game } = audio;
  const change = (patch: Partial<FullDemoOptions['audio']>): void => onChange({ ...options, audio: { ...audio, ...patch } });
  const status = fullDemoVoiceStatus(document?.voice.availability ?? null);
  // A demo known to lack voices says so even with the box off; otherwise the box already tells.
  const availability = voice.enabled || status.problem ? status : null;
  return <FullDemoGroup title="Sonido" note="Audio de partida equilibrado automáticamente, sin música de fondo.">
    <FullDemoToggle label={FULL_DEMO_VOICE_LABEL} value={voice.enabled} onChange={(enabled) => change({ voice: { ...voice, enabled } })} />
    {availability ? <p className={availability.problem ? 'text-body-sm text-warning' : 'text-body-sm text-fg-2'} role="status">{availability.text}</p> : null}
    <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
      <FullDemoGain label="Juego" value={game.gain} onChange={(gain) => change({ game: { ...game, gain } })} />
      <FullDemoGain label="Voces" value={voice.gain} disabled={!voice.enabled} onChange={(gain) => change({ voice: { ...voice, gain } })} />
    </div>
  </FullDemoGroup>;
}
