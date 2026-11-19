import { currentEdition } from '@/lib/edition';

/** What the Replay says for each team voice error code; the server's own text stays in studio.log. */
const VOICE_ERRORS: Readonly<Record<string, string>> = {
  voice_unavailable: 'La voz del equipo necesita FFmpeg, que esta instalación de Studio no tiene.',
  no_voice: 'Esta partida no tiene voz del equipo: importa una grabación primero.',
  not_audio: 'No es una grabación de audio que Studio pueda leer.',
  too_many_tracks: 'Una partida guarda como mucho 16 pistas de voz.',
  transcriber_not_installed: 'Instala la transcripción local antes de transcribir.',
  queue_full: 'Hay demasiadas transcripciones en cola. Vuelve a intentarlo en un rato.',
  unsupported: 'La transcripción local solo funciona en Windows de 64 bits.',
  no_demo_file: 'El archivo de la demo no está en este PC, así que Studio no puede leer su voz.',
  runtime_missing:
    'La transcripción no arranca porque falta Microsoft Visual C++ Redistributable 2015-2022 (x64). Instálalo y vuelve a intentarlo.',
  start_failed:
    'La transcripción no arranca en este PC. Instala Microsoft Visual C++ Redistributable 2015-2022 (x64) y vuelve a intentarlo.',
  download_failed: 'No se pudo descargar la transcripción. Comprueba la conexión y vuelve a intentarlo.',
};

const WEB_VOICE_UNAVAILABLE = 'La voz del equipo se importa en ClipHub Studio y llega aquí al sincronizar.';

export function voiceErrorText(code: string | undefined, fallback: string): string {
  if (code === 'voice_unavailable' && currentEdition() === 'web') return WEB_VOICE_UNAVAILABLE;
  return (code !== undefined && VOICE_ERRORS[code]) || fallback;
}
