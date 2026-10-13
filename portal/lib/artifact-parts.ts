import type { ArtifactKind } from "./job-types.ts";

export const ARTIFACT_NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.(mp4|jpg)$/;
export const ARTIFACT_VARIANT_PATTERN = /^[a-z0-9-]{1,40}$/;
export const MAX_ARTIFACTS_PER_JOB = 40;

export function isArtifactName(name: string, kind: ArtifactKind): boolean {
  if (!ARTIFACT_NAME_PATTERN.test(name)) return false;
  return name.endsWith(kind === "video" ? ".mp4" : ".jpg");
}

export function partCount(sizeBytes: number, partSize: number): number {
  return Math.max(1, Math.ceil(sizeBytes / partSize));
}

export interface PartInput {
  sizeBytes: number;
  partSize: number;
  // 1-based.
  part: number;
}

// Bytes part n must carry, or null when n is not a part of the file.
export function expectedPartLength(input: PartInput): number | null {
  const { sizeBytes, partSize, part } = input;
  const count = partCount(sizeBytes, partSize);
  if (!Number.isInteger(part) || part < 1 || part > count) return null;
  return part < count ? partSize : sizeBytes - (count - 1) * partSize;
}

export function missingParts(count: number, received: readonly number[]): number[] {
  const have = new Set(received);
  const missing: number[] = [];
  for (let part = 1; part <= count; part += 1) {
    if (!have.has(part)) missing.push(part);
  }
  return missing;
}

// Reads the receivedParts column: a JSON array of 1-based part numbers.
export function parseReceivedParts(raw: string | null): number[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const parts = parsed.filter(
      (part): part is number =>
        typeof part === "number" && Number.isInteger(part) && part > 0,
    );
    return [...new Set(parts)].sort((a, b) => a - b);
  } catch {
    return [];
  }
}

export function addReceivedPart(raw: string | null, part: number): number[] {
  return [...new Set([...parseReceivedParts(raw), part])].sort((a, b) => a - b);
}

const DAY_MS = 24 * 60 * 60 * 1000;
// How long a file Studio already has stays on the portal.
const RECEIVED_RETENTION_MS = DAY_MS;

// When a result file stops being downloadable: the retention window after its
// upload, or 24 hours after Studio confirmed it has the file, whichever is first.
export function artifactExpiresAt(
  artifact: { uploadedAt: Date; receivedAt: number | null },
  retentionDays: number,
): number {
  const byAge = artifact.uploadedAt.getTime() + retentionDays * DAY_MS;
  if (artifact.receivedAt === null) return byAge;
  return Math.min(byAge, artifact.receivedAt + RECEIVED_RETENTION_MS);
}
