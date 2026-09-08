'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions, FullDemoTransitionOptions } from '@/lib/full-demo-plan';
import { DEFAULT_FULL_DEMO_TRANSITIONS, fullDemoTransitionPreset } from '@/lib/full-demo-transitions';
import { Button } from '@/components/ui/button';
import { FullDemoChoice, FullDemoGroup, FullDemoNumber, FullDemoToggle } from './full-demo-fields';

export function FullDemoTransitions({ options, onChange }: { options: FullDemoOptions; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const value = options.transitions ?? DEFAULT_FULL_DEMO_TRANSITIONS;
  function change(patch: Partial<FullDemoTransitionOptions>): void {
    onChange({ ...options, transitions: { ...value, ...patch } });
  }
  return <FullDemoGroup title="Entre rondas" note="Marca el cambio de ronda con movimiento y sonido. Conserva el tiempo de juego y los 2 segundos de preparación.">
    <FullDemoToggle label="Activar efectos entre rondas" value={value.enabled} onChange={(enabled) => change({ enabled })} />
    {value.enabled ? <div className="min-w-0 space-y-5">
      <div className="flex flex-wrap gap-2" role="group" aria-label="Ajustes rápidos de transición">
        {([{ id: 'subtle', label: 'Suave' }, { id: 'kinetic', label: 'Dinámico' }, { id: 'audio', label: 'Solo sonido' }] as const).map(({ id, label }) =>
          <Button key={id} type="button" variant="secondary" size="sm" onClick={() => onChange({ ...options, transitions: fullDemoTransitionPreset(id) })}>{label}</Button>)}
      </div>
      <FullDemoNumber label="Duración de la transición (fotogramas)" min={6} max={18} value={value.duration_frames} onChange={(duration_frames) => change({ duration_frames })} />
      <p className="text-meta text-fg-3">{Math.round(value.duration_frames / 60 * 1000)} ms a 60 fps. La duración se reparte a ambos lados del corte, sin solapar ni quitar fotogramas.</p>
      <section className="space-y-3" aria-label="Efectos visuales entre rondas">
        <h3 className="font-display text-body font-semibold uppercase text-fg-1">Movimiento e imagen</h3>
        <div className="grid gap-x-4 sm:grid-cols-2">
          <FullDemoToggle label="Barrido direccional (whip-pan)" value={value.whip} onChange={(whip) => change({ whip })} />
          <FullDemoToggle label="Zoom rápido (punch-in)" value={value.zoom} onChange={(zoom) => change({ zoom })} />
          <FullDemoToggle label="Microflash de brillo" value={value.flash} onChange={(flash) => change({ flash })} />
          <FullDemoToggle label="Separación RGB" value={value.rgb_split} onChange={(rgb_split) => change({ rgb_split })} />
        </div>
        {value.whip ? <div className="grid gap-4 sm:grid-cols-2">
          <FullDemoChoice label="Dirección del barrido" value={value.direction} options={[
            { value: 'alternate', label: 'Alternar izquierda y derecha' }, { value: 'follow-motion', label: 'Seguir movimiento de la imagen' },
            { value: 'left', label: 'Izquierda' }, { value: 'right', label: 'Derecha' }, { value: 'up', label: 'Arriba' }, { value: 'down', label: 'Abajo' },
          ]} onChange={(direction) => change({ direction })} />
          <FullDemoNumber label="Fuerza del barrido (%)" min={2} max={20} value={Math.round(value.whip_strength * 100)} onChange={(percent) => change({ whip_strength: percent / 100 })} />
          <FullDemoNumber label="Desenfoque direccional (px)" max={64} value={value.blur_pixels} onChange={(blur_pixels) => change({ blur_pixels })} />
          {value.direction === 'follow-motion' ? <p className="self-center text-meta text-fg-3">Estima el movimiento al final del plano; alterna la dirección cuando la imagen está quieta.</p> : null}
        </div> : null}
        {value.zoom ? <div className="grid gap-4 sm:grid-cols-2">
          <FullDemoNumber label="Aumento del zoom (%)" min={5} max={15} step={1} value={value.zoom_percent} onChange={(zoom_percent) => change({ zoom_percent })} />
          <FullDemoChoice label="Momento del zoom" value={value.zoom_anchor} options={[
            { value: 'cut', label: 'En el cambio de ronda' }, { value: 'last-kill', label: 'Última baja del jugador' }, { value: 'round-end', label: 'Fin de la ronda' },
          ]} onChange={(zoom_anchor) => change({ zoom_anchor })} />
          {value.zoom_anchor !== 'cut' ? <p className="text-meta text-fg-3 sm:col-span-2">Si ese evento queda fuera del plano o no hay baja, el zoom se aplica en el corte.</p> : null}
        </div> : null}
        {value.flash || value.rgb_split ? <div className="grid gap-4 sm:grid-cols-2">
          <FullDemoNumber label={value.flash ? 'Duración del microflash (fotogramas)' : 'Duración del acento RGB (fotogramas)'} min={2} max={4} value={value.flash_frames} onChange={(flash_frames) => change({ flash_frames })} />
          {value.flash ? <FullDemoNumber label="Intensidad del microflash (%)" min={2} max={20} value={Math.round(value.flash_intensity * 100)} onChange={(percent) => change({ flash_intensity: percent / 100 })} /> : null}
          {value.flash && value.rgb_split ? <p className="text-meta text-fg-3 sm:col-span-2">El microflash y el acento RGB comparten estos fotogramas alrededor del corte.</p> : null}
        </div> : null}
        {value.rgb_split ? <FullDemoNumber label="Separación de color (px)" min={1} max={6} value={value.rgb_pixels} onChange={(rgb_pixels) => change({ rgb_pixels })} /> : null}
      </section>
      <section className="space-y-3 border-t border-border-subtle pt-4" aria-label="Sonido entre rondas">
        <h3 className="font-display text-body font-semibold uppercase text-fg-1">Sonido del corte</h3>
        <div className="grid gap-x-4 sm:grid-cols-2">
          <FullDemoToggle label="Whoosh suave" value={value.whoosh} onChange={(whoosh) => change({ whoosh })} />
          <FullDemoToggle label="Impacto grave al entrar" value={value.impact} onChange={(impact) => change({ impact })} />
          {value.whoosh ? <FullDemoNumber label="Volumen del whoosh (dB)" min={-36} max={-6} value={value.whoosh_gain_db} onChange={(whoosh_gain_db) => change({ whoosh_gain_db })} /> : null}
          {value.impact ? <FullDemoNumber label="Volumen del impacto (dB)" min={-36} max={-6} value={value.impact_gain_db} onChange={(impact_gain_db) => change({ impact_gain_db })} /> : null}
        </div>
        <FullDemoNumber label="Cola de voces hacia la siguiente ronda (s)" max={1.5} step={.1} value={value.comms_tail_seconds} onChange={(comms_tail_seconds) => change({ comms_tail_seconds })} />
        <p className="text-meta text-fg-3">Continúa las voces de equipo que siguen al corte y las desvanece sobre la preparación siguiente. 0 desactiva la cola.{!options.audio.voice.enabled ? ' Activa las voces del jugador para oírla.' : ''}</p>
        <details className="space-y-3">
          <summary className="cursor-pointer text-body-sm text-fg-2">Ajustar mezcla del corte</summary>
          <div className="grid gap-4 sm:grid-cols-2">
            <FullDemoNumber label="Fundido del juego (ms)" max={250} step={5} value={value.game_fade_ms} onChange={(game_fade_ms) => change({ game_fade_ms })} />
            <FullDemoNumber label="Filtro grave de salida (Hz; 0 desactiva)" max={12000} step={200} value={value.game_tail_lowpass_hz} onChange={(game_tail_lowpass_hz) => change({ game_tail_lowpass_hz })} />
            {value.impact ? <>
              <FullDemoNumber label="Duración del impacto (ms)" min={80} max={400} step={10} value={value.impact_duration_ms} onChange={(impact_duration_ms) => change({ impact_duration_ms })} />
              <FullDemoNumber label="Frecuencia del impacto (Hz)" min={35} max={90} value={value.impact_frequency} onChange={(impact_frequency) => change({ impact_frequency })} />
            </> : null}
          </div>
        </details>
      </section>
      <p className="text-meta text-fg-3">Los anuncios mantienen su entrada y salida propias. Los efectos se guardan con el plan del vídeo.</p>
    </div> : null}
  </FullDemoGroup>;
}
