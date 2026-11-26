import { FULL_DEMO_VOICE_LABEL } from './full-demo-brief.ts';

export type FullDemoNotice = { readonly code: string; readonly message: string; readonly round_id?: string };

const TURN_VOICES_OFF = `Quita «${FULL_DEMO_VOICE_LABEL}» para crear el vídeo.`;

export const FULL_DEMO_ERROR_CODES: Readonly<Record<string, string>> = {
  full_demo_facts_insufficient: 'Faltan las jugadas analizadas del jugador. Vuelve a elegirlo en el paso Jugador.',
  invalid_full_demo_options: 'Algún ajuste no es válido. Revísalos y vuelve a intentarlo.',
  invalid_full_demo_document: 'El plan guardado no se puede leer. Vuelve a preparar la demo completa.',
  full_demo_plan_stale: 'El plan cambió mientras lo preparabas. Vuelve a intentarlo.',
  generate_work_active: 'Studio está grabando o montando esta partida. Espera a que termine y vuelve a intentarlo.',
};

const BLOCKER_TEXT: Readonly<Record<string, string>> = {
  voice_unavailable: `Esta demo no tiene voces del equipo que se puedan usar. ${TURN_VOICES_OFF}`,
  voice_decode_failed: `No se pudieron leer las voces de esta demo. ${TURN_VOICES_OFF}`,
  full_demo_asset_missing: 'Falta un archivo del vídeo o no se pudo verificar. Vuelve a elegir la intro, el sponsor, el outro o la música.',
  full_demo_facts_insufficient: 'La demo no tiene rondas completas que grabar desde este jugador. Elige otro jugador o importa otra demo de la partida.',
  audio_master_validation_failed: 'El audio del vídeo no pasó la comprobación. Vuelve a intentarlo.',
};

const WARNING_TEXT: Readonly<Record<string, string>> = {
  voice_unavailable: 'El vídeo se hace sin las voces del equipo.',
  full_demo_facts_insufficient: 'Una ronda se queda fuera: la demo no tiene su principio y su final completos.',
  pov_dead_in_freeze: 'Una ronda se queda fuera: el jugador murió antes de empezar a jugarla.',
  pov_tail_trimmed: 'El final de una ronda se acorta para no pasar de la siguiente.',
  sponsor_after_last_round: 'El vídeo tiene una sola ronda, así que el sponsor sale después de ella.',
  hook_skipped: 'El vídeo sale sin gancho inicial: hace falta más de una ronda y alguna baja del jugador.',
  crosshair_capture_default_approved: 'En alguna ronda la demo no guarda la mira del jugador y se graba la mira por defecto.',
};

const SPANISH_ASSET_MESSAGE = /^(Sube la captura|La captura de)/;

function roundSuffix(roundId: string | undefined): string {
  if (roundId === undefined || roundId === '') return '';
  const number = /(\d+)$/.exec(roundId)?.[1];
  return number === undefined ? '' : ` Ronda ${Number(number)}.`;
}

export function fullDemoBlockerText(notice: FullDemoNotice): string {
  if (notice.code === 'pov_contract_failed') return notice.message;
  if (notice.code === 'full_demo_asset_missing' && SPANISH_ASSET_MESSAGE.test(notice.message)) return notice.message;
  const text = BLOCKER_TEXT[notice.code] ?? 'Algo impide crear este vídeo. Revisa los ajustes y vuelve a intentarlo.';
  return `${text}${roundSuffix(notice.round_id)}`;
}

export function fullDemoWarningText(notice: FullDemoNotice): string {
  if (notice.code === 'pov_contract_failed') return notice.message;
  const text = WARNING_TEXT[notice.code];
  if (text === undefined) return '';
  return `${text}${roundSuffix(notice.round_id)}`;
}
