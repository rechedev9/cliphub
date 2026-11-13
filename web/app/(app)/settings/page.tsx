import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { AssistantSettings } from '@/components/settings/assistant-settings';
import { AppearanceSettings } from '@/components/settings/appearance-settings';
import { StudioInfo } from '@/components/settings/studio-info';
import { SteamAccount } from '@/components/settings/steam-account';
import { ClipMindAccount } from '@/components/settings/clipmind-account';
import { TelemetrySettings } from '@/components/settings/telemetry-settings';
import { TeamSpeakSettings } from '@/components/settings/teamspeak-settings';
import { CaptureReadiness } from '@/components/shell/capture-readiness';
import { PlaybackDiagnostics } from '@/components/settings/playback-diagnostics';
import { currentEdition } from '@/lib/edition';

/** The web has no PC to prepare: only the look; the account and team have their own pages. */
function WebSettings(): ReactNode {
  return (
    <div className="flex flex-col gap-7">
      <StudioPageHeader title="Ajustes" description="Elige cómo se ve ClipMind. Tu cuenta y tu equipo están en Cuenta y Equipo." />
      <div className="measure-work flex flex-col gap-7">
        <AppearanceSettings />
      </div>
    </div>
  );
}

/** Studio's settings; the web edition shows only what applies to it. */
export default function SettingsPage(): ReactNode {
  if (currentEdition() === 'web') return <WebSettings />;
  return (
    <div className="flex flex-col gap-7">
      <StudioPageHeader
        title="Ajustes"
        description="Prepara este PC para grabar y gestiona tus conexiones opcionales."
      />
      <div className="measure-work flex flex-col gap-7">
        <CaptureReadiness variant="settings" />
        <SteamAccount />
        <ClipMindAccount />
        <AppearanceSettings />
        <StudioInfo />
        <AssistantSettings />
        <TeamSpeakSettings />
        <PlaybackDiagnostics />
        <TelemetrySettings />
      </div>
    </div>
  );
}
