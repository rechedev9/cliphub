import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StudioInfo } from '@/components/settings/studio-info';
import { SteamAccount } from '@/components/settings/steam-account';
import { TelemetrySettings } from '@/components/settings/telemetry-settings';
import { CaptureReadiness } from '@/components/shell/capture-readiness';

/** Desktop-only application settings. */
export default function SettingsPage(): ReactNode {
  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="Ajustes"
        description="Prepara este PC para grabar y gestiona las conexiones opcionales de ClipHub."
      />
      <div className="measure-read flex flex-col gap-8">
        <section id="capture" className="scroll-mt-20"><CaptureReadiness variant="settings" /></section>
        <p className="text-body-sm text-fg-2">Conectar Steam solo es necesario para importar partidas desde tu cuenta. Puedes cargar demos y archivos MP4 sin configurarlo.</p>
        <SteamAccount />
        <StudioInfo />
        <TelemetrySettings />
      </div>
    </div>
  );
}
