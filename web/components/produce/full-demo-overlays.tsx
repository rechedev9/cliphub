'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { FULL_DEMO_SOURCE_LABEL, FULL_DEMO_SOURCES } from '@/lib/produce/full-demo-brief';
import { FullDemoChoice } from './full-demo-fields';

export function FullDemoOverlays({ options, map, onChange, onAssetBusy }: { options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void }): ReactNode {
  void map; void onAssetBusy;
  return <div className="min-w-0">
    <FullDemoChoice label={FULL_DEMO_SOURCE_LABEL} value={options.source_kind} options={FULL_DEMO_SOURCES} onChange={(source_kind) => onChange({ ...options, source_kind })} />
  </div>;
}
