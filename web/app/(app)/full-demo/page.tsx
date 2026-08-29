import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { FullDemoPicker } from '@/components/full-demo/demo-picker';
import { FULL_DEMO_CONTRACT } from '@/lib/full-demo';

export default function FullDemoIndexPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="DEMO COMPLETA A VÍDEO"
        description="Sube una demo, elige el jugador y revisa la configuración antes de capturar toda su POV. La salida es horizontal, con HUD nativo y comms; sin música."
      />
      <dl className="studio-panel grid gap-x-6 gap-y-2 px-4 py-4 text-body-sm sm:grid-cols-2">
        {FULL_DEMO_CONTRACT.map((row) => (
          <div key={row.label} className="flex min-w-0 gap-2">
            <dt className="shrink-0 text-fg-3">{row.label}</dt>
            <dd className="truncate text-fg-1">{row.value}</dd>
          </div>
        ))}
      </dl>
      <FullDemoPicker />
    </div>
  );
}
