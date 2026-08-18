import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StudioInfo } from '@/components/settings/studio-info';
import { SteamAccount } from '@/components/settings/steam-account';


/** Desktop-only application settings. */
export default function SettingsPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="CONFIGURACIÓN"
        description="Consulta la versión instalada y conecta la cuenta de Steam con la que juegas."
      />
      {/* Bound like the empty state; a full-width dl would be a band of labels. */}
      <div className="flex max-w-2xl flex-col gap-8">
        <StudioInfo />
        <SteamAccount />
      </div>
    </div>
  );
}
