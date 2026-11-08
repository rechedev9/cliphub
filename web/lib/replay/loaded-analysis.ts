import { fetchTacticalDocument, fetchTacticalPositions, isPositionsMissingError, isServiceUnavailableError } from '../api/tactical.ts';
import type { TacticalDocument, TacticalFrame, TacticalRound, TacticalSource } from '../api/tactical.ts';
import { decodePositionsHeader, decodeRoundFrames, TacticalDecodeError, tacticalDecodeErrorMessage } from '../tactical-decode.ts';
import type { PositionsScale } from '../tactical-decode.ts';
import { LruCache } from '../lru-cache.ts';

/** Rounds kept decoded per match: the superposed rounds plus a few neighbours. */
const FRAME_CACHE_ROUNDS = 14;

/** What a replay says when the analysis is there but its positions file is not. */
export const POSITIONS_MISSING_MESSAGE =
  'Falta el archivo de posiciones de esta demo, así que no se puede reproducir. Vuelve a analizar la demo para reconstruirlo.';

/** A message the analysis screens can show as is. */
export class UserFacingTacticalError extends Error {}

/**
 * One match's tactical document and the position bytes it describes. Rounds are
 * decoded out of those bytes on demand, so changing round is a seek, never a request.
 */
export class LoadedAnalysis {
  readonly doc: TacticalDocument;
  readonly #blob: ArrayBuffer;
  readonly #scale: PositionsScale;
  readonly #frames = new LruCache<number, TacticalFrame[]>(FRAME_CACHE_ROUNDS);

  constructor(doc: TacticalDocument, blob: ArrayBuffer) {
    this.doc = doc;
    this.#blob = blob;
    this.#scale = decodePositionsHeader(blob);
  }

  round(number: number): TacticalRound | undefined {
    return this.doc.rounds.find((round) => round.number === number);
  }

  /** The round's frames; a truncated or mis-offset round reads as empty instead of throwing. */
  framesFor(number: number): TacticalFrame[] {
    const cached = this.#frames.get(number);
    if (cached !== undefined) return cached;
    const offset = this.doc.positions.round_offsets.find((entry) => entry.round === number);
    if (offset === undefined) return [];
    try {
      const decoded = decodeRoundFrames(this.#blob, offset, this.#scale);
      this.#frames.set(number, decoded);
      return decoded;
    } catch {
      return [];
    }
  }
}

/**
 * Verifies the blob against the digest the document recorded: the index and
 * the bytes must come from one scan. Without `crypto.subtle` the check is skipped.
 */
async function positionsAreStale(doc: TacticalDocument, blob: ArrayBuffer): Promise<boolean> {
  const expected = doc.positions.sha256;
  if (!expected || typeof crypto === 'undefined' || crypto.subtle === undefined) return false;
  const digest = await crypto.subtle.digest('SHA-256', blob);
  const actual = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
  return actual !== expected.toLowerCase();
}

/** Fetches the document and its positions together and checks they belong to each other. */
export async function loadAnalysis(source: TacticalSource): Promise<LoadedAnalysis> {
  const [doc, blob] = await Promise.all([fetchTacticalDocument(source), fetchTacticalPositions(source)]);
  if (await positionsAreStale(doc, blob)) {
    throw new UserFacingTacticalError(
      'Las posiciones no coinciden con el índice (SHA-256 distinto): el análisis está desfasado y hay que repetirlo.',
    );
  }
  return new LoadedAnalysis(doc, blob);
}

export function analysisErrorMessage(error: unknown): string {
  if (isServiceUnavailableError(error)) return 'El servicio de análisis local no está disponible.';
  if (isPositionsMissingError(error)) return POSITIONS_MISSING_MESSAGE;
  if (error instanceof TacticalDecodeError) return tacticalDecodeErrorMessage(error);
  if (error instanceof UserFacingTacticalError) return error.message;
  return 'No se pudo cargar el análisis táctico. Revisa el estado local y vuelve a intentarlo.';
}
