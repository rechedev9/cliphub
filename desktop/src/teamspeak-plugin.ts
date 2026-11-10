/** The TeamSpeak 3 plugin package `scripts/build-teamspeak-plugin.ps1` builds; assemble ships it in `bin`. */
export const TEAMSPEAK_PLUGIN_FILE = 'cliphub_teamspeak.ts3_plugin';

export type TeamSpeakInstallResult = { ok: true } | { ok: false; error: string };

export interface TeamSpeakInstallOptions {
  packagePath: string;
  exists: (file: string) => boolean;
  /** Electron's shell.openPath: resolves to an empty string on success, else the reason. */
  openPath: (file: string) => Promise<string>;
  logLine: (line: string) => void;
}

/** Opens the package with TeamSpeak 3, whose installer registers .ts3_plugin and asks the user to confirm. */
export async function openTeamSpeakPlugin(options: TeamSpeakInstallOptions): Promise<TeamSpeakInstallResult> {
  if (!options.exists(options.packagePath)) {
    options.logLine(`[teamspeak] plugin package missing: ${options.packagePath}\n`);
    return { ok: false, error: 'Esta instalación de Studio no incluye el plugin de TeamSpeak. Reinstala Studio.' };
  }
  const failure = await options.openPath(options.packagePath).catch((error: unknown) => String(error));
  if (failure === '') return { ok: true };
  options.logLine(`[teamspeak] could not open the plugin package: ${failure}\n`);
  return { ok: false, error: 'Windows no sabe abrir el plugin. Instala TeamSpeak 3 y vuelve a pulsar el botón.' };
}
