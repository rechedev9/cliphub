import { CUSTOM_HUD_THEMES, verticalHudTheme } from './custom-hud.ts';
import type { DesignGalleryItem } from './design-gallery.ts';
import { hudPreviewUrl, hudStyleOf, sameHudStyle, verticalHudPreviewUrl, type HudStyle, type UserHudTheme } from './hud-themes.ts';
import type { OverlayLanguage } from './overlay-language.ts';

/** Own themes are gallery items too; the prefix keeps their ids apart from the catalog's. */
export const OWN_HUD_PREFIX = 'user:';

/** The catalog, then the own themes. The theme being edited stays chosen while its colours differ
 * from the saved ones; a style no saved theme matches marks its catalog base. */
export function fullDemoHudGallery({ base, style, themes, editingId }: {
  base: string; style?: HudStyle; themes: readonly UserHudTheme[]; editingId?: string | null;
}): { items: DesignGalleryItem[]; editing?: UserHudTheme } {
  const matched = style ? themes.find((theme) => theme.base === base && sameHudStyle(hudStyleOf(theme), style)) : undefined;
  const editing = style ? themes.find((theme) => theme.id === (editingId ?? matched?.id)) : undefined;
  const items: DesignGalleryItem[] = [
    ...CUSTOM_HUD_THEMES.map((theme) => ({ id: theme.id, label: theme.name, image: `/hud/${theme.id}.webp`, selected: !editing && theme.id === base })),
    ...themes.map((theme) => ({
      id: `${OWN_HUD_PREFIX}${theme.id}`, label: theme.name, note: 'Tema propio', selected: theme.id === editing?.id,
      image: hudPreviewUrl({ base: theme.base, style: hudStyleOf(theme) }),
    })),
  ];
  return { items, editing };
}

/** The Short scoreboard takes only catalog designs, each shown as the band the video draws. */
export function verticalHudGallery(selected: string | undefined, language?: OverlayLanguage): DesignGalleryItem[] {
  const chosen = verticalHudTheme(selected).id;
  return CUSTOM_HUD_THEMES.map((theme) => ({ id: theme.id, label: theme.name, image: verticalHudPreviewUrl(theme.id, language), selected: theme.id === chosen }));
}
