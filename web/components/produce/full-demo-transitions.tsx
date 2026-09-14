'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { fullDemoTransitionPreset } from '@/lib/full-demo-transitions';
import { FullDemoGroup, FullDemoToggle } from './full-demo-fields';

export function FullDemoTransitions({ options, onChange }: { options: FullDemoOptions; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const enabled = options.transitions?.enabled ?? true;
  return <FullDemoGroup title="Entre rondas" note="Transición dinámica entre rondas.">
    <FullDemoToggle label="Activar efectos entre rondas" value={enabled} onChange={(next) => onChange({ ...options, transitions: { ...fullDemoTransitionPreset(), enabled: next } })} />
  </FullDemoGroup>;
}
