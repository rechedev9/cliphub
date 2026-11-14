/** A team's HTML document (an anti-strat or prep report) and how the browser reads, uploads and shows it. */
export interface TeamDocument {
  id: string;
  team_id: string;
  team_name: string;
  title: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
  updated_at: string;
}

export const TEAM_DOCUMENT_KEYS = ['id', 'team_id', 'team_name', 'title', 'file_name', 'content_type', 'size_bytes', 'created_at', 'updated_at'] as const;

/** The server's cap on one document, and on the whole upload with its form fields. */
export const TEAM_DOCUMENT_MAX_BYTES = 5 * 1024 * 1024;
export const TEAM_DOCUMENT_REQUEST_BYTES = TEAM_DOCUMENT_MAX_BYTES + 64 * 1024;
export const TEAM_DOCUMENT_TITLE_MAX = 120;
export const TEAM_DOCUMENT_ACCEPT = '.html,.htm,text/html';

/**
 * The policy a document is served under, equal to TeamDocumentPolicy in
 * internal/httpapi/library_team_documents.go: an opaque origin, so its scripts never reach the session or the API.
 */
export const TEAM_DOCUMENT_POLICY =
  "sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox; default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' https:; style-src 'unsafe-inline' https:; font-src https: data:; img-src https: data: blob:; media-src https: data: blob:; connect-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'self'";

/** The iframe's own sandbox: scripts and links that open a new tab, never the app's origin. */
export const TEAM_DOCUMENT_SANDBOX = 'allow-scripts allow-popups allow-popups-to-escape-sandbox';

export function teamDocumentContentHref(id: string): string {
  return `/api/library/documents/${encodeURIComponent(id)}/content`;
}

/** Why a picked file cannot be uploaded, before it leaves the browser; null when it can. */
export function teamDocumentFileProblem(file: { name: string; size: number; type: string }): string | null {
  const name = file.name.toLowerCase();
  if (!name.endsWith('.html') && !name.endsWith('.htm') && file.type !== 'text/html') return 'Solo se admiten documentos HTML (.html o .htm).';
  if (file.size === 0) return 'El archivo está vacío.';
  if (file.size > TEAM_DOCUMENT_MAX_BYTES) return 'El documento pasa de 5 MB. Deja las imágenes más ligeras o enlázalas.';
  return null;
}

/** The title a file gets by default: its name without the extension. */
export function defaultTeamDocumentTitle(fileName: string): string {
  return fileName.replace(/\.html?$/i, '').trim().slice(0, TEAM_DOCUMENT_TITLE_MAX);
}

export function formatDocumentSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} MB`;
}

async function errorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body: unknown = await res.json();
    if (body !== null && typeof body === 'object') {
      const code: unknown = Reflect.get(body, 'code');
      if (code === 'document_too_large' || res.status === 413) return 'El documento pasa de 5 MB.';
      if (code === 'invalid_document') return 'El archivo no es un documento HTML en UTF-8.';
    }
  } catch {
    // A body that is not JSON keeps the fallback.
  }
  if (res.status === 404) return 'El equipo o el documento ya no existe.';
  if (res.status === 401) return 'La sesión ha caducado. Vuelve a iniciar sesión.';
  return fallback;
}

export async function listTeamDocuments(teamId: string, signal?: AbortSignal): Promise<TeamDocument[]> {
  const res = await fetch(`/api/library/documents?team_id=${encodeURIComponent(teamId)}&limit=200`, { signal, cache: 'no-store' });
  if (!res.ok) throw new Error(await errorMessage(res, 'No se pudieron leer los documentos.'));
  const body: unknown = await res.json();
  const documents: unknown = body !== null && typeof body === 'object' ? Reflect.get(body, 'documents') : undefined;
  return Array.isArray(documents) ? documents.filter(isTeamDocument) : [];
}

export async function uploadTeamDocument(input: { teamId: string; title: string; file: File }): Promise<TeamDocument> {
  const form = new FormData();
  form.append('team_id', input.teamId);
  if (input.title.trim() !== '') form.append('title', input.title.trim());
  form.append('file', input.file, input.file.name);
  const res = await fetch('/api/library/document-uploads', { method: 'POST', body: form });
  if (!res.ok) throw new Error(await errorMessage(res, 'No se pudo subir el documento.'));
  const body: unknown = await res.json();
  if (!isTeamDocument(body)) throw new Error('El servidor devolvió una respuesta inesperada.');
  return body;
}

export async function deleteTeamDocument(id: string): Promise<void> {
  const res = await fetch(`/api/library/documents/${encodeURIComponent(id)}`, { method: 'DELETE' });
  if (!res.ok && res.status !== 404) throw new Error(await errorMessage(res, 'No se pudo borrar el documento.'));
}

function isTeamDocument(value: unknown): value is TeamDocument {
  if (value === null || typeof value !== 'object') return false;
  return (
    typeof Reflect.get(value, 'id') === 'string' &&
    typeof Reflect.get(value, 'team_id') === 'string' &&
    typeof Reflect.get(value, 'title') === 'string' &&
    typeof Reflect.get(value, 'file_name') === 'string' &&
    typeof Reflect.get(value, 'size_bytes') === 'number' &&
    typeof Reflect.get(value, 'updated_at') === 'string'
  );
}
