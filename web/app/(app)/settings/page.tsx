import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StudioInfo } from '@/components/settings/studio-info';
import { SteamAccount } from '@/components/settings/steam-account';
import { TelemetrySettings } from '@/components/settings/telemetry-settings';

/** Desktop-only application settings. */
export default function SettingsPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="CONFIGURACIÓN"
        description="Consulta la versión, controla los diagnósticos y conecta la cuenta de Steam con la que juegas."
      />
      <div className="measure-read flex flex-col gap-8">
        <StudioInfo />
        <TelemetrySettings />
        <SteamAccount />
      </div>
    </div>
  );
}
