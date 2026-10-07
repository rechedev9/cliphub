/** Shown when team voice is on and this demo has no voice packets. */
export const FULL_DEMO_NO_VOICE_PACKETS =
  'Esta demo no tiene datos de voz. Desactiva «Incluir voces del equipo» para crear el vídeo.';

/**
 * The planner keeps the English availability sentence (`Team voice is unavailable: no_packets`).
 * The long-video form is the place that sentence becomes an instruction.
 */
export function fullDemoBlockerText(notice: { code: string; message: string }): string {
  if (notice.code === 'voice_unavailable' && notice.message.endsWith(': no_packets')) {
    return FULL_DEMO_NO_VOICE_PACKETS;
  }
  return notice.message;
}
