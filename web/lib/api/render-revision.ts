const UUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
const EMPTY_UUID = '00000000-0000-0000-0000-000000000000';

export function renderRevisionFromPrefix(prefix: string | undefined, jobId: string, variant: string): string | undefined {
  const base = `jobs/${jobId}/renders/${variant}/revisions/`;
  if (!prefix?.startsWith(base)) return undefined;
  const revision = prefix.slice(base.length);
  return UUID.test(revision) && revision !== EMPTY_UUID ? revision : undefined;
}

/** An absent revision keeps legacy URLs; a supplied invalid revision is rejected. */
export function requestedRenderRevision(request: Request): string | null {
  const values = new URL(request.url).searchParams.getAll('revision');
  if (values.length === 0) return '';
  const value = values[0];
  return values.length === 1 && UUID.test(value) && value !== EMPTY_UUID ? `/revisions/${value}` : null;
}

export function pinRenderRevision(url: string, revision: string | undefined): string {
  return revision ? `${url}?revision=${encodeURIComponent(revision)}` : url;
}
