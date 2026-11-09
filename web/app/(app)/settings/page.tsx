import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { AssistantSettings } from '@/components/settings/assistant-settings';
import { AppearanceSettings } from '@/components/settings/appearance-settings';
import { StudioInfo } from '@/components/settings/studio-info';
import { SteamAccount } from '@/components/settings/steam-account';
import { TelemetrySettings } from '@/components/settings/telemetry-settings';
import { CaptureReadiness } from '@/components/shell/capture-readiness';
import { PlaybackDiagnostics } from '@/components/settings/playback-diagnostics';

/** Desktop-only application settings. */
export default function SettingsPage(): ReactNode {
  return (
    <div className="flex flex-col gap-7">
      <StudioPageHeader
        title="Ajustes"
        description="Prepara este PC para grabar y gestiona tus conexiones opcionales."
      />
      <div className="measure-work flex flex-col gap-7">
        <CaptureReadiness variant="settings" />
        <SteamAccount />
        <AppearanceSettings />
        <StudioInfo />
        <AssistantSettings />
        <PlaybackDiagnostics />
        <TelemetrySettings />
      </div>
    </div>
  );
}
