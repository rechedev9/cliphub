import {
  archiveExtractError,
  archiveHasNoDemosError,
  archivePasswordError,
  isArchiveFileName,
  isDemoFileName,
  isRarFileName,
  MAX_DEMO_FILES,
  notDemoOrArchiveError,
  tooManyDemosError,
} from './demo-names.ts';
import { extractZipDemos } from './extract-zip.ts';

export type ExpandDemoUploadsResult = { ok: true; files: File[] } | { ok: false; error: string };

const ZIP_LOCAL = 0x04034b50;
const ZIP_CENTRAL = 0x02014b50;
const ZIP_EOCD = 0x06054b50;
const RAR_MARK = [0x52, 0x61, 0x72, 0x21, 0x1a, 0x07] as const;

export async function expandDemoUploads(files: readonly File[]): Promise<ExpandDemoUploadsResult> {
  const out: File[] = [];
  for (const file of files) {
    if (isDemoFileName(file.name)) {
      out.push(file);
      continue;
    }
    if (!isArchiveFileName(file.name)) {
      return { ok: false, error: notDemoOrArchiveError(file.name) };
    }
    const extracted = await extractArchiveDemos(file);
    if (!extracted.ok) return extracted;
    if (extracted.files.length === 0) {
      return { ok: false, error: archiveHasNoDemosError(file.name) };
    }
    out.push(...extracted.files);
  }
  if (out.length === 0) return { ok: false, error: archiveHasNoDemosError(files[0]?.name ?? 'archivo') };
  if (out.length > MAX_DEMO_FILES) return { ok: false, error: tooManyDemosError(out.length) };
  return { ok: true, files: out };
}

async function extractArchiveDemos(file: File): Promise<ExpandDemoUploadsResult> {
  try {
    const bytes = new Uint8Array(await file.arrayBuffer());
    const kind = sniffArchive(bytes) ?? (isRarFileName(file.name) ? 'rar' : 'zip');
    const demos = kind === 'zip' ? extractZipDemos(bytes) : await extractRar(bytes);
    return { ok: true, files: demos };
  } catch (err) {
    if (isPasswordError(err)) return { ok: false, error: archivePasswordError(file.name) };
    return { ok: false, error: archiveExtractError(file.name) };
  }
}

async function extractRar(bytes: Uint8Array): Promise<File[]> {
  const { extractRarDemos } = await import('./extract-rar.ts');
  return extractRarDemos(toArrayBuffer(bytes));
}

function sniffArchive(data: Uint8Array): 'zip' | 'rar' | null {
  if (data.length >= 4) {
    const sig = data[0] | (data[1] << 8) | (data[2] << 16) | (data[3] << 24);
    if (sig === ZIP_LOCAL || sig === ZIP_CENTRAL || sig === ZIP_EOCD) return 'zip';
  }
  if (data.length >= RAR_MARK.length && RAR_MARK.every((byte, i) => data[i] === byte)) return 'rar';
  return null;
}

function toArrayBuffer(data: Uint8Array): ArrayBuffer {
  const copy = new Uint8Array(data.byteLength);
  copy.set(data);
  return copy.buffer;
}

const PASSWORD_REASONS = new Set(['ERAR_MISSING_PASSWORD', 'ERAR_BAD_PASSWORD']);

function isPasswordError(err: unknown): boolean {
  if (typeof err !== 'object' || err === null) return false;
  if (err instanceof Error && err.name === 'ArchivePasswordError') return true;
  if (!('reason' in err) || typeof err.reason !== 'string') return false;
  return PASSWORD_REASONS.has(err.reason);
}
