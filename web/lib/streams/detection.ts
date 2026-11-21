import type { StreamVoiceScan, StreamVoiceSettings } from '../api/streams.ts';

type ShoutOption = { value: string; label: string };

const SHOUT_PRESETS: readonly ShoutOption[] = [
  { value: '5', label: 'Poco' },
  { value: '3', label: 'Normal' },
  { value: '1', label: 'Mucho' },
];

export function shoutOptions(sensitivity: number): ShoutOption[] {
  if (SHOUT_PRESETS.some((preset) => preset.value === String(sensitivity))) return [...SHOUT_PRESETS];
  return [...SHOUT_PRESETS, { value: String(sensitivity), label: `Nivel ${sensitivity}` }].sort((a, b) => Number(b.value) - Number(a.value));
}

const DEFAULT_VOICE: StreamVoiceSettings = { enabled: true, sensitivity: 3 };

export function scanVoiceSettings(scanned: StreamVoiceScan | undefined): StreamVoiceSettings {
  return scanned ? { enabled: scanned.enabled, sensitivity: scanned.sensitivity } : { ...DEFAULT_VOICE };
}

export function detectionSettingsChanged(scanned: StreamVoiceScan | undefined, voice: StreamVoiceSettings): boolean {
  if (scanned === undefined) return true;
  return scanned.enabled !== voice.enabled || (voice.enabled && scanned.sensitivity !== voice.sensitivity);
}

export function detectionNote(voice: StreamVoiceScan | undefined): string | null {
  switch (voice?.status) {
    case 'no_audio':
      return 'El vídeo no tiene audio, así que no hemos buscado picos de voz.';
    case 'no_voice':
      return 'No hemos oído voz en el audio, así que no hay picos de voz.';
    case 'failed':
      return 'No pudimos leer el audio para buscar picos de voz.';
    case 'done':
      return voice.mix ? 'No encontramos tu micro por separado y escuchamos el audio del juego con tu voz, así que los picos de voz salen menos precisos.' : null;
    default:
      return null;
  }
}
