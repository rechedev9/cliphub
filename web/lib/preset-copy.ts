export const NATIVE_HUD_LABEL = 'Nativo CS2 (radar, vida, killfeed)';

export function presetDescription(preset: {
  name: string;
  description: string;
  descriptionEs?: string;
}): string {
  const spanish = preset.descriptionEs?.trim();
  return spanish ? spanish : preset.description;
}
