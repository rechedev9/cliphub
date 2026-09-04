/** Hub copy shared with the E2E contract; a plain module so specs can import it. */
export const HUB_EMPTY_TITLE = '¿Qué quieres crear?';

/** `unpicked` row: roster scanned, nobody chose a POV. The CTA resumes the pick on `/clips/nueva?job=`. */
export const MATCH_ROW_UNPICKED_TITLE = 'Sin POV elegida';
export const MATCH_ROW_UNPICKED_HINT = 'roster listo · elige el jugador a clipear';
export const MATCH_ROW_UNPICKED_CTA = 'Elegir jugador';

/** Partidas lens section for reels whose job is no longer listed. */
export const HUB_ORPHANS_TITLE = 'Otros clips';
export const HUB_ORPHANS_HINT = 'Su partida ya no está en la lista: la demo se borró o el trabajo falló.';

/** Hub header once at least one partida exists; the empty state keeps its own title. */
export const HUB_TITLE = 'Tus demos y vídeos';
export const HUB_DESCRIPTION =
  'Continúa con una partida, descarga tus vídeos o empieza una nueva creación.';
export const HUB_LOAD_DEMO_CTA = 'Cargar demo';

/** `ready` row with nothing produced yet: the next step is the first Short. */
export const MATCH_ROW_FIRST_CLIP_CTA = 'Crear Short';

/** Three-step first-run guide, shown until every step is done or the user hides it. */
export const FIRST_RUN_GUIDE_TITLE = 'Cómo funciona';
export const FIRST_RUN_GUIDE_DISMISS = 'Ocultar guía';
export const FIRST_RUN_STEPS = {
  load: { title: 'Carga una demo', hint: 'Un .dem de CS2. Se parsea en este PC, nada sale de tu equipo.' },
  pick: { title: 'Elige tu POV', hint: 'El jugador que quieres clipear. ClipHub busca sus mejores jugadas.' },
  produce: { title: 'Saca tu primer clip', hint: 'Shorts en vertical o el Full POV con HUD nativo.' },
} as const;
