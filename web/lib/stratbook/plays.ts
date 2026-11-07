import {
  TACTICAL_CT_PATTERN_ORDER,
  TACTICAL_T_PATTERN_ORDER,
  type TacticalSide,
  type TacticalSite,
} from '../api/tactical.ts';
import {
  LIBRETA_SECTIONS,
  STRATBOOK_FORMAT,
  STRATBOOK_VERSION,
  type DetectedPlay,
  type LibraryPlay,
  type LineupStatus,
  type PlayGrenadeKind,
  type PlayInput,
  type PlayOrigin,
  type LibretaSection,
  type SectionImport,
  type StratbookImport,
  type StratbookUpload,
} from '../api/stratbook.ts';
import { patternLabel, siteLabel } from '../tactical-labels.ts';

/** The most example rounds a play keeps (`library.MaxPlayExamples`). */
export const MAX_PLAY_EXAMPLES = 50;

export const PLAY_ORIGIN_LABEL: Record<PlayOrigin, string> = {
  round: 'Guardada de una ronda',
  detected: 'Detectada como repetida',
  import: 'Importada de un fichero',
  manual: 'Escrita a mano',
  playbook: 'Combinación del playbook',
};

export const GRENADE_LABEL: Record<PlayGrenadeKind, string> = {
  smoke: 'Humo',
  flash: 'Flash',
  molotov: 'Molotov',
  he: 'HE',
};

export const LINEUP_STATUS_LABEL: Record<LineupStatus, string> = {
  matched: 'En el catálogo',
  unmatched: 'No coincide con ningún lineup',
  no_catalog: 'Sin lineups de este mapa',
  not_covered: 'El catálogo solo tiene humos',
};

/** The kinds a play of one side can take: that side's round patterns. */
export function playKinds(side: TacticalSide): readonly string[] {
  return side === 'T' ? TACTICAL_T_PATTERN_ORDER : TACTICAL_CT_PATTERN_ORDER;
}

export function playKindLabel(kind: string): string {
  return patternLabel(kind);
}

/** "Ejecución A", or the kind alone when the play has no site. */
export function playShapeLabel(play: { kind: string; site: TacticalSite }): string {
  const kind = playKindLabel(play.kind);
  return play.site === 'none' ? kind : `${kind} ${siteLabel(play.site)}`;
}

/** A starting title for a play saved from a round. */
export function suggestPlayTitle(play: { kind: string; site: TacticalSite; side: TacticalSide }, round: number): string {
  return `${playShapeLabel(play)} (${play.side}, ronda ${round})`;
}

/** The share of a detected play's rounds, as a whole percent. */
export function shareLabel(share: number | undefined): string {
  return share === undefined ? '' : `${Math.round(share * 100)} %`;
}

/** Seconds after freeze end, as the replay clock counts them. */
export function playSecondsLabel(seconds: number): string {
  return `${seconds.toFixed(1).replace('.', ',')} s`;
}

/** A play saved from a round draft, with the coach's title and notes. */
export function playFromDraft(draft: LibraryPlay, title: string, notes: string): PlayInput {
  return {
    title: title.trim(),
    team_id: draft.team_id,
    map: draft.map,
    side: draft.side,
    kind: draft.kind,
    site: draft.site,
    origin: 'round',
    positions_seconds: draft.positions_seconds,
    grenades: draft.grenades,
    positions: draft.positions,
    notes: notes.trim(),
    examples: draft.examples.slice(0, MAX_PLAY_EXAMPLES),
  };
}

/** A play saved from a repeated play the detection found, with all its rounds as examples. */
export function playFromDetected(play: DetectedPlay, input: { teamId: string; title: string; positionSeconds: number }): PlayInput {
  return {
    title: input.title.trim(),
    team_id: input.teamId,
    map: play.map,
    side: play.side,
    kind: play.kind,
    site: play.site,
    origin: 'detected',
    positions_seconds: play.positions.length === 0 ? 0 : input.positionSeconds,
    grenades: play.grenades,
    positions: play.positions,
    notes: '',
    examples: play.examples.slice(0, MAX_PLAY_EXAMPLES),
  };
}

export type StratbookParse = { ok: true; file: StratbookUpload } | { ok: false; error: string };

/** Reads a stratbook file chosen by the user; the orchestrator validates every play again. */
export function parseStratbookFile(text: string): StratbookParse {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { ok: false, error: 'El fichero no es JSON válido.' };
  }
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return { ok: false, error: 'El fichero no es una Libreta de ClipHub.' };
  }
  const format: unknown = Reflect.get(value, 'format');
  const version: unknown = Reflect.get(value, 'version');
  const plays: unknown = Reflect.get(value, 'plays');
  if (format !== STRATBOOK_FORMAT || !Array.isArray(plays)) {
    return { ok: false, error: 'El fichero no es una Libreta de ClipHub.' };
  }
  if (typeof version !== 'number' || !Number.isInteger(version) || version < 1 || version > STRATBOOK_VERSION) {
    return { ok: false, error: `Este fichero es de la versión ${String(version)} y Studio lee hasta la ${STRATBOOK_VERSION}. Actualiza Studio.` };
  }
  const exportedAt: unknown = Reflect.get(value, 'exported_at');
  const file: StratbookUpload = {
    format: STRATBOOK_FORMAT,
    version,
    exported_at: typeof exportedAt === 'string' ? exportedAt : new Date(0).toISOString(),
    plays,
  };
  for (const section of LIBRETA_SECTIONS) {
    const records: unknown = Reflect.get(value, section);
    if (Array.isArray(records) && records.length > 0) file[section] = records;
  }
  return { ok: true, file };
}

/** "libreta-vitality-2026-10-30.json": the team when the export is one team's, and the day. */
export function stratbookFileName(date: Date, teamName?: string): string {
  const slug = (teamName ?? '')
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40)
    .replace(/-+$/g, '');
  const day = date.toISOString().slice(0, 10);
  return slug === '' ? `libreta-${day}.json` : `libreta-${slug}-${day}.json`;
}

const SECTION_LABEL: Record<LibretaSection, { label: string; feminine: boolean }> = {
  lineups: { label: 'Lineups', feminine: false },
  team_roles: { label: 'Hojas de roles', feminine: true },
  calls: { label: 'Llamadas', feminine: true },
  callout_names: { label: 'Nombres de callouts', feminine: false },
};

function sectionSentence(section: LibretaSection, counts: SectionImport): string | null {
  const { label, feminine } = SECTION_LABEL[section];
  const parts: string[] = [];
  if (counts.imported > 0) parts.push(`${counts.imported} ${feminine ? 'importada' : 'importado'}${counts.imported === 1 ? '' : 's'}`);
  if (counts.duplicates > 0) parts.push(`${counts.duplicates} ${counts.duplicates === 1 ? 'ya estaba' : 'ya estaban'} en la Libreta`);
  if (counts.rejected > 0) parts.push(`${counts.rejected} no se ${counts.rejected === 1 ? 'pudo' : 'pudieron'} leer`);
  return parts.length === 0 ? null : `${label}: ${parts.join(', ')}.`;
}

/** What an import did, in Spanish: the plays, then each other section the file held, in tab order. */
export function importSummary(result: StratbookImport): string {
  const sections = LIBRETA_SECTIONS.flatMap((section) => {
    const counts = result.sections?.[section];
    const sentence = counts === undefined ? null : sectionSentence(section, counts);
    return sentence === null ? [] : [sentence];
  });
  const parts: string[] = [];
  // A whole-Libreta file without plays says nothing about plays.
  if (sections.length === 0 || result.imported + result.duplicates + result.rejected.length > 0) {
    parts.push(result.imported === 1 ? 'Importada 1 jugada.' : `Importadas ${result.imported} jugadas.`);
  }
  if (result.duplicates > 0) parts.push(result.duplicates === 1 ? '1 ya estaba en la Libreta.' : `${result.duplicates} ya estaban en la Libreta.`);
  const rejected = result.rejected.length;
  if (rejected > 0) {
    const titles = result.rejected.map((entry) => entry.title || `n.º ${entry.index + 1}`).join(', ');
    parts.push(`${rejected} no se ${rejected === 1 ? 'pudo' : 'pudieron'} leer: ${titles}.`);
  }
  return [...parts, ...sections].join(' ');
}
