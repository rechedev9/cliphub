'use client';

import type { ReactNode } from 'react';
import type { FullDemoDocument, FullDemoOptions } from '@/lib/full-demo-plan';
import { FullDemoGain, FullDemoGroup, FullDemoToggle } from './full-demo-fields';

type Props = { options: FullDemoOptions; document: FullDemoDocument | null; onChange: (options: FullDemoOptions) => void };

export function FullDemoAudio({ options, document, onChange }: Props): ReactNode {
  const { audio } = options;
  const { voice, game } = audio;
  const change = (patch: Partial<FullDemoOptions['audio']>): void => onChange({ ...options, audio: { ...audio, ...patch } });
  // The checkbox already says when voices are off; the line only reports what the demo holds.
  const availability = voice.enabled ? voiceStatus(document?.voice.availability ?? null) : null;
  return <FullDemoGroup title="Sonido" note="Audio de partida equilibrado automáticamente, sin música de fondo.">
    <FullDemoToggle label="Incluir voces del equipo" value={voice.enabled} onChange={(enabled) => change({ voice: { ...voice, enabled } })} />
    {availability ? <p className={availability.problem ? 'text-body-sm text-warning' : 'text-body-sm text-fg-2'} role="status">{availability.text}</p> : null}
    <div className="flex flex-col gap-2.5 border-t border-border-subtle pt-3">
      <FullDemoGain label="Juego" value={game.gain} onChange={(gain) => change({ game: { ...game, gain } })} />
      <FullDemoGain label="Voces" value={voice.gain} disabled={!voice.enabled} onChange={(gain) => change({ voice: { ...voice, gain } })} />
    </div>
  </FullDemoGroup>;
}

const VOICE_PENDING = 'Las voces del equipo se comprueban al preparar el vídeo.';
const VOICE_STATUS: Record<string, { text: string; problem: boolean }> = {
  available: { text: 'Voces del equipo disponibles en esta demo.', problem: false },
  // Planned while voices were off: the next preparation checks them.
  not_requested: { text: VOICE_PENDING, problem: false },
  no_packets: { text: 'Esta demo no contiene voces.', problem: true },
  no_team_voice: { text: 'Esta demo no tiene voces del equipo del jugador.', problem: true },
  silent: { text: 'Las voces del equipo están en silencio en esta demo.', problem: true },
  unsupported: { text: 'El formato de voz de esta demo no es compatible.', problem: true },
  unsupported_codec: { text: 'El códec de voz de esta demo no es compatible.', problem: true },
  decode_failed: { text: 'No se pudieron leer las voces de esta demo.', problem: true },
};

/** `null` means no plan has analysed the demo yet. */
function voiceStatus(status: string | null): { text: string; problem: boolean } {
  if (status === null) return { text: VOICE_PENDING, problem: false };
  return VOICE_STATUS[status] ?? { text: `Estado de las voces: ${status}.`, problem: true };
}
