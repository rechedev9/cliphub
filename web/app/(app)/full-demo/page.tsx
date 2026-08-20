import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { FullDemoPicker } from '@/components/full-demo/demo-picker';
import { FullDemoStylePicker } from '@/components/full-demo/style-picker';
import { FULL_DEMO_CONTRACT, FULL_DEMO_VARIANT } from '@/lib/full-demo';

export default function FullDemoIndexPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="FULL DEMO TO VIDEO"
        description="POV landscape 16:9: rondas en vivo sin freeze, HUD nativo y comms del equipo. Sin música. Suelta una demo o elige una ya parseada; el brief de Shorts no aplica aquí."
      />
      <dl className="studio-panel grid gap-x-6 gap-y-2 px-4 py-4 text-body-sm sm:grid-cols-2">
        {FULL_DEMO_CONTRACT.map((row) => (
          <div key={row.label} className="flex min-w-0 gap-2">
            <dt className="shrink-0 text-fg-3">{row.label}</dt>
            <dd className="truncate text-fg-1">{row.value}</dd>
          </div>
        ))}
      </dl>
      <FullDemoStylePicker value={FULL_DEMO_VARIANT} />
      <FullDemoPicker />
    </div>
  );
}
