'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { fullDemoTransitionPreset } from '@/lib/full-demo-transitions';
import { FULL_DEMO_TRANSITIONS_LABEL } from '@/lib/produce/full-demo-brief';
import { FullDemoFold, FullDemoToggle } from './full-demo-fields';

export function FullDemoTransitions({ options, onChange }: { options: FullDemoOptions; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const enabled = options.transitions?.enabled ?? true;
  return <FullDemoFold title="Entre rondas" value={enabled ? 'Con efectos' : 'Sin efectos'} note="Transición dinámica entre rondas.">
    <FullDemoToggle label={FULL_DEMO_TRANSITIONS_LABEL} value={enabled} onChange={(next) => onChange({ ...options, transitions: { ...fullDemoTransitionPreset(), enabled: next } })} />
  </FullDemoFold>;
}
