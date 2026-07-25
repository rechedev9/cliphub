import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StudioInfo } from '@/components/settings/studio-info';


/** Desktop-only application settings. */
export default function SettingsPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="CONFIGURACIÓN"
        description="Consulta la versión instalada de FragForge Studio. El agente integrado usa tu sesión personal de Codex."
      />
      {/*
        A single spec sheet, bounded like the shared empty state rather than
        stretched across the full content column: a four-row `<dl>` at 1440px
        would be a band of whitespace with four labels floating in it.
      */}
      <div className="max-w-2xl">
        <StudioInfo />
      </div>
    </div>
  );
}
