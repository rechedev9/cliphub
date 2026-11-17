'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions, FullDemoProPlayer } from '@/lib/full-demo-plan';
import { FULL_DEMO_PRO_SETTINGS_LABEL, OWN_PRO_SETTINGS_LABEL, proResolutionLabel, proViewmodelLabel, withProPlayer } from '@/lib/produce/full-demo-pro-settings';
import { FullDemoChoice, FullDemoPart } from './full-demo-fields';

const OWN = 'own';
const NOTE = 'Elige un pro para grabar con su resolución y su viewmodel. La mira siempre es la del jugador de la demo. Al terminar, CS2 vuelve a tu configuración.';

export function FullDemoProSettings({ options, players, onChange }: { options: FullDemoOptions; players: readonly FullDemoProPlayer[]; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const selected = options.capture.pro_settings;
  const choices = [{ value: OWN, label: OWN_PRO_SETTINGS_LABEL }, ...players.map((player) => ({ value: player.id, label: `${player.name} · ${player.team}` }))];
  const source = players.find((player) => player.id === selected?.player_id);
  return <FullDemoPart title="Configuración de pro" note={NOTE}>
    <FullDemoChoice label={FULL_DEMO_PRO_SETTINGS_LABEL} value={selected?.player_id ?? OWN} options={choices}
      onChange={(id) => onChange(withProPlayer(options, players.find((player) => player.id === id) ?? null))} />
    {selected ? <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-body-sm">
      <dt className="text-fg-3">Resolución</dt><dd className="text-fg-1">{proResolutionLabel(selected)}</dd>
      <dt className="text-fg-3">Viewmodel</dt><dd className="text-fg-1">{proViewmodelLabel(selected)}</dd>
      {source ? <><dt className="text-fg-3">Fuente</dt><dd className="text-fg-2">prosettings.net, {source.updated}</dd></> : null}
    </dl> : null}
    {selected && selected.scaling === 'stretched' && selected.capture_width < 1920
      ? <p className="text-body-sm text-fg-2">El vídeo se estira a 16:9, como lo ve el pro en su pantalla.</p> : null}
  </FullDemoPart>;
}
