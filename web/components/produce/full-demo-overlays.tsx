'use client';

import type { ReactNode } from 'react';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { FullDemoChoice } from './full-demo-fields';

export function FullDemoOverlays({ options, map, onChange, onAssetBusy }: { options: FullDemoOptions; map: string; onChange: (options: FullDemoOptions) => void; onAssetBusy: (busy: boolean) => void }): ReactNode {
  void map; void onAssetBusy;
  return <div className="min-w-0">
    <FullDemoChoice label="Origen de la demo" value={options.source_kind} options={[{ value: 'faceit', label: 'FACEIT' }, { value: 'premier', label: 'Premier' }, { value: 'professional', label: 'Profesional' }, { value: 'demo', label: 'Demo local' }]} onChange={(source_kind) => onChange({ ...options, source_kind })} />
  </div>;
}
