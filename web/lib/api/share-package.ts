import { LibraryApiError } from './library.ts';
import { SHARE_PACKAGE_EXTENSION, type SharePackageImport } from '../library/share.ts';

export type ShareTarget = { kind: 'report'; id: string };
export type SharePackageFile = { blob: Blob; fileName: string };

const FILE_NAME = /filename="([^"]+)"/;

async function failure(res: Response): Promise<LibraryApiError> {
  const body: unknown = await res.json().catch(() => null);
  const error: unknown = body !== null && typeof body === 'object' ? Reflect.get(body, 'error') : undefined;
  const code: unknown = body !== null && typeof body === 'object' ? Reflect.get(body, 'code') : undefined;
  return new LibraryApiError(
    typeof error === 'string' ? error : `share request failed (${res.status})`,
    res.status,
    typeof code === 'string' ? code : undefined,
  );
}

/** Builds the .cliphub package of a report. */
export async function exportSharePackage(target: ShareTarget, notes: string): Promise<SharePackageFile> {
  const res = await fetch(`/api/library/reports/${encodeURIComponent(target.id)}/export`, {
    method: 'POST',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ notes }),
  });
  if (!res.ok) throw await failure(res);
  const name = FILE_NAME.exec(res.headers.get('content-disposition') ?? '')?.[1] ?? `cliphub${SHARE_PACKAGE_EXTENSION}`;
  return { blob: await res.blob(), fileName: name };
}

/** Opens a .cliphub package into the library. */
export async function importSharePackage(file: File): Promise<SharePackageImport> {
  const form = new FormData();
  form.append('package', file, file.name);
  const res = await fetch('/api/library/import-package', { method: 'POST', cache: 'no-store', body: form });
  if (!res.ok) throw await failure(res);
  const body: SharePackageImport = await res.json();
  return body;
}

/** Hands the package to the browser's save flow; Studio shows the Windows save dialog. */
export function savePackageFile(file: SharePackageFile): void {
  const url = URL.createObjectURL(file.blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = file.fileName;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
