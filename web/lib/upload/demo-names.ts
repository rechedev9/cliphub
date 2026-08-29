const DEM_EXT = '.dem';
const DEM_ZST_EXT = '.dem.zst';
const RAR_EXT = '.rar';
const ZIP_EXT = '.zip';
export const MAX_DEMO_FILES = 10;

export function isDemoFileName(name: string): boolean {
  const lower = name.toLowerCase();
  return lower.endsWith(DEM_EXT) || lower.endsWith(DEM_ZST_EXT);
}

function isZipFileName(name: string): boolean {
  return name.toLowerCase().endsWith(ZIP_EXT);
}

export function isRarFileName(name: string): boolean {
  return name.toLowerCase().endsWith(RAR_EXT);
}

export function isArchiveFileName(name: string): boolean {
  return isZipFileName(name) || isRarFileName(name);
}

export function archiveEntryBaseName(path: string): string {
  const normalized = path.replaceAll('\\', '/');
  const parts = normalized.split('/');
  return parts[parts.length - 1] ?? path;
}

export function isIgnoredArchivePath(path: string): boolean {
  const normalized = path.replaceAll('\\', '/');
  if (normalized.split('/').includes('__MACOSX')) return true;
  const base = archiveEntryBaseName(normalized);
  return base.startsWith('.') || base.length === 0;
}

export function tooManyDemosError(count: number): string {
  return `Máximo ${MAX_DEMO_FILES} demos por serie. Has soltado ${count}.`;
}

export function notDemoOrArchiveError(name: string): string {
  return `"${name}" no es una demo .dem / .dem.zst ni un archivo .rar / .zip.`;
}

export function archiveHasNoDemosError(name: string): string {
  return `"${name}" no contiene ninguna demo .dem o .dem.zst.`;
}

export function archiveExtractError(name: string): string {
  return `No se pudo extraer "${name}".`;
}

export function archivePasswordError(name: string): string {
  return `"${name}" está protegido con contraseña.`;
}
