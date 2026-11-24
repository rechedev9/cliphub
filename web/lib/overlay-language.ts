export const OVERLAY_LANGUAGES = ['es', 'en'] as const;
export type OverlayLanguage = (typeof OVERLAY_LANGUAGES)[number];

export const OVERLAY_LANGUAGE_LABEL: Readonly<Record<OverlayLanguage, string>> = { es: 'Español', en: 'Inglés' };

export function isOverlayLanguage(value: unknown): value is OverlayLanguage {
  return value === 'es' || value === 'en';
}

export function englishOverlays(edit: { overlayLanguage?: OverlayLanguage | undefined }): boolean {
  return edit.overlayLanguage === 'en';
}

export type PreviewTexts = {
  title: string;
  winProbability: string;
  ctPlate: string;
  tPlate: string;
  matchStats: string;
  statLines: readonly string[];
};

export function previewTexts(language: OverlayLanguage | undefined, mapName: string): PreviewTexts {
  if (language === 'en') {
    return {
      title: `3K ON ${mapName}`,
      winProbability: 'WIN PROBABILITY',
      ctPlate: 'CT 64%',
      tPlate: '36% T',
      matchStats: 'MATCH STATS',
      statLines: ['K/D 1.45 (16/11)', 'ADR 92.4 · HS 48%', 'CLIPHUB RATING 1.21'],
    };
  }
  return {
    title: `3K EN ${mapName}`,
    winProbability: 'PROB. DE VICTORIA',
    ctPlate: 'CT 64 %',
    tPlate: '36 % T',
    matchStats: 'ESTADÍSTICAS DE LA PARTIDA',
    statLines: ['K/D 1.45 (16/11)', 'ADR 92.4 · HS 48 %', 'RATING DE CLIPHUB 1.21'],
  };
}
