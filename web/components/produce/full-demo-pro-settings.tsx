'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions, FullDemoProPlayer } from '@/lib/full-demo-plan';
import { proResolutionLabel, proViewmodelLabel, withProPlayer } from '@/lib/produce/full-demo-pro-settings';
import { FullDemoChoice, FullDemoGroup } from './full-demo-fields';

const OWN = 'own';
const NOTE = 'Graba con la resolución y el viewmodel del pro. La mira ya es la suya, sale de la demo. Al terminar, tu CS2 vuelve a tu configuración.';

export function FullDemoProSettings({ options, players, onChange }: { options: FullDemoOptions; players: readonly FullDemoProPlayer[]; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const selected = options.capture.pro_settings;
  const choices = [{ value: OWN, label: 'Los tuyos (1920x1080)' }, ...players.map((player) => ({ value: player.id, label: `${player.name} · ${player.team}` }))];
  const source = players.find((player) => player.id === selected?.player_id);
  return <FullDemoGroup title="Settings de pro" note={NOTE}>
    <FullDemoChoice label="Settings de la grabación" value={selected?.player_id ?? OWN} options={choices}
      onChange={(id) => onChange(withProPlayer(options, players.find((player) => player.id === id) ?? null))} />
    {selected ? <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-body-sm">
      <dt className="text-fg-3">Resolución</dt><dd className="text-fg-1">{proResolutionLabel(selected)}</dd>
      <dt className="text-fg-3">Viewmodel</dt><dd className="text-fg-1">{proViewmodelLabel(selected)}</dd>
      {source ? <><dt className="text-fg-3">Fuente</dt><dd className="text-fg-2">prosettings.net, {source.updated}</dd></> : null}
    </dl> : null}
    {selected && selected.scaling === 'stretched' && selected.capture_width < 1920
      ? <p className="text-body-sm text-fg-2">El vídeo se estira a 16:9, como lo ve el pro en su pantalla.</p> : null}
  </FullDemoGroup>;
}
