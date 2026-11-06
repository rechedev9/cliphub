/**
 * Copy for share packages (`internal/sharepkg`): what a .cliphub file carries
 * and what an import did, in the words the dialogs show.
 */

export const SHARE_PACKAGE_EXTENSION = '.cliphub';
export const SHARE_FILE = /\.cliphub$/i;
/** `sharepkg.MaxNotesRunes`. */
export const SHARE_NOTES_MAX = 4000;

/** Mirrors `httpapi.shareImportResponse`. */
export type SharePackageImport = {
  kind: 'collection' | 'report';
  title: string;
  notes: string;
  collection_id?: string;
  report_id?: string;
  duplicate: boolean;
  matches_imported: number;
  matches_existing: number;
  annotations: number;
};

export const SHARE_IMPORT_KEYS = [
  'kind', 'title', 'notes', 'collection_id', 'report_id', 'duplicate', 'matches_imported', 'matches_existing', 'annotations',
] as const;

const SHARE_ERROR_LABEL: Record<string, string> = {
  share_empty: 'Ninguna de sus partidas sigue en la biblioteca.',
  share_match_unavailable: 'Una de sus partidas no tiene el análisis guardado. Vuelve a analizarla y prueba otra vez.',
  share_package_invalid: 'El archivo no es un paquete de ClipHub válido o está dañado.',
  share_demo_pending: 'Una de sus demos ya está en tu biblioteca y aún se está analizando. Espera a que termine y vuelve a importarlo.',
};

export function shareErrorLabel(code: string | undefined, fallback: string): string {
  return (code === undefined ? undefined : SHARE_ERROR_LABEL[code]) ?? fallback;
}

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

/** File size the way the Windows explorer reads it, with a decimal comma. */
export function packageSizeLabel(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  const mb = bytes / (1024 * 1024);
  return `${mb.toLocaleString('es-ES', { maximumFractionDigits: mb < 10 ? 1 : 0 })} MB`;
}

/** What a collection package will carry, before it is built. */
export function collectionShareSummary(moments: number, matches: number): string {
  return `Lleva ${plural(moments, 'momento', 'momentos')} de ${plural(matches, 'partida', 'partidas')} con sus rondas, posiciones y notas, sin las demos.`;
}

/** What a report package will carry, before it is built. */
export function reportShareSummary(matches: number): string {
  return `Lleva el informe y ${plural(matches, 'partida', 'partidas')} con sus rondas, posiciones y notas, sin las demos.`;
}

/** One line on what an import added to the library. */
export function shareImportSummary(result: SharePackageImport): string {
  const what = result.kind === 'collection' ? 'La colección' : 'El informe';
  if (result.duplicate) return `${what} «${result.title}» ya estaba en tu biblioteca.`;
  const parts = [];
  if (result.matches_imported > 0) parts.push(plural(result.matches_imported, 'partida nueva', 'partidas nuevas'));
  if (result.matches_existing > 0) parts.push(`${plural(result.matches_existing, 'partida', 'partidas')} que ya tenías`);
  if (result.annotations > 0) parts.push(plural(result.annotations, 'nota', 'notas'));
  const added = parts.length === 0 ? '' : `: ${parts.join(', ')}`;
  return `${what} «${result.title}» está en tu biblioteca${added}.`;
}
