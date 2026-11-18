import { customHudTheme } from './custom-hud.ts';
import type { DesignGalleryItem } from './design-gallery.ts';
import type { FullDemoOptions } from './full-demo-plan.ts';

type Overlays = FullDemoOptions['overlays'];

const PREVIEW_API = '/api/full-demo/overlay-designs/preview';

/** The title of the selector's part, shared with the summary. */
export const INTRO_DESIGN_LABEL = 'Presentación y marcador';
export const INTRO_DESIGNS = { neon: 'Neón', hud: 'A juego con el HUD' } as const;

/** The orchestrator draws the example match with the renderer's own page; `hud` takes the plan's HUD and own theme. */
export function introDesignPreviewUrl({ design, overlays, outro = false }: { design: keyof typeof INTRO_DESIGNS; overlays: Overlays; outro?: boolean }): string {
  const query = new URLSearchParams({ design });
  if (design === 'hud' && overlays.hud_theme) {
    query.set('base', overlays.hud_theme);
    const style = overlays.hud_custom;
    if (style) {
      query.set('ct', style.ct); query.set('t', style.t); query.set('background', style.background);
      if (style.radius !== undefined) query.set('radius', String(style.radius));
    }
  }
  if (outro) query.set('outro', '1');
  return `${PREVIEW_API}?${query.toString()}`;
}

/** Neón, the default, and the design in the HUD's colours; ids are the plan's `overlays.theme`. */
export function introDesignGallery(overlays: Overlays): DesignGalleryItem[] {
  const hudName = overlays.hud_custom?.name ?? customHudTheme(overlays.hud_theme)?.name;
  return [
    { id: 'neon-violet', label: INTRO_DESIGNS.neon, image: introDesignPreviewUrl({ design: 'neon', overlays }), selected: overlays.theme !== 'hud' },
    {
      id: 'hud', label: INTRO_DESIGNS.hud, image: introDesignPreviewUrl({ design: 'hud', overlays }), selected: overlays.theme === 'hud',
      note: hudName ? `Colores de ${hudName}` : 'Tonos neutros con el HUD de CS2',
    },
  ];
}
