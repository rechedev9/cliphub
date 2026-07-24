import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { XAISettings } from '@/components/settings/xai-settings';
import { StudioInfo } from '@/components/settings/studio-info';


/** Desktop-only application settings. Secret handling remains in Electron. */
export default function SettingsPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="CONFIGURACIÓN"
        description="Consulta la versión instalada y configura las credenciales opcionales de subtítulos. El agente integrado usa tu sesión personal de Codex."
      />
      {/*
        Two columns keyed to the content container, not the viewport: the
        actionable credential panel on the left, the build spec sheet as a
        narrow aside. Both panels fill their track, so their right edges align
        by construction — v3 stacked one `max-w-3xl` panel under an
        unconstrained one and the mismatch read as a ragged edge.
      */}
      <div className="grid items-start gap-6 @[58rem]/content:grid-cols-[minmax(0,1.6fr)_minmax(16rem,0.85fr)]">
        <XAISettings />
        <StudioInfo />
      </div>
    </div>
  );
}
