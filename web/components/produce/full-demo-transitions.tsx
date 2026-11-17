'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { fullDemoTransitionPreset } from '@/lib/full-demo-transitions';
import { FULL_DEMO_TRANSITIONS_LABEL } from '@/lib/produce/full-demo-brief';
import { FullDemoPart, FullDemoToggle } from './full-demo-fields';

export function FullDemoTransitions({ options, onChange }: { options: FullDemoOptions; onChange: (value: FullDemoOptions) => void }): ReactNode {
  const enabled = options.transitions?.enabled ?? true;
  return <FullDemoPart title="Entre rondas" note="Transición dinámica entre rondas.">
    <FullDemoToggle label={FULL_DEMO_TRANSITIONS_LABEL} value={enabled} onChange={(next) => onChange({ ...options, transitions: { ...fullDemoTransitionPreset(), enabled: next } })} />
  </FullDemoPart>;
}
