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
import { SettingsIndex, type SettingsIndexEntry } from '@/components/settings/settings-index';
import { currentEdition } from '@/lib/edition-routes';
import { VOICE_SETTINGS_ANCHOR } from '@/lib/nav-groups';

const STUDIO_SECTIONS: readonly SettingsIndexEntry[] = [
  { id: 'capture', label: 'Grabación' },
  { id: 'steam', label: 'Steam' },
  { id: 'clipmind', label: 'ClipMind' },
  { id: 'appearance', label: 'Apariencia' },
  { id: 'studio', label: 'Versión' },
  { id: 'assistants', label: 'Asistentes' },
  { id: VOICE_SETTINGS_ANCHOR, label: 'Voz del equipo' },
  { id: 'playback', label: 'Motor de vídeo' },
  { id: 'telemetry-settings', label: 'Diagnósticos' },
];

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
      <SettingsIndex entries={STUDIO_SECTIONS} />
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
