import type { EditorDocument, EditorSample } from '../editor/evaluate.ts';
import { agentAwareFetch } from './agent-fetch.ts';

export const EDITOR_STATUS = {
  draft: 'draft',
  rendering: 'rendering',
  rendered: 'rendered',
  failed: 'failed',
} as const;

export type EditorStatus = (typeof EDITOR_STATUS)[keyof typeof EDITOR_STATUS];

export type EditorAsset = {
  id: string;
  sha256: string;
  file_name: string;
  origin: string;
  probe: {
    width?: number;
    height?: number;
    duration_seconds?: number;
    has_audio?: boolean;
  };
  created_at: string;
};

export type EditorProject = {
  id: string;
  title: string;
  status: EditorStatus;
  failure_reason?: string;
  plan?: EditorDocument;
  created_at: string;
  updated_at: string;
};

export type EditorRenderState = {
  project_id: string;
  status: EditorStatus | 'draft';
  fingerprint?: string;
  warnings?: string[];
  error?: string;
};

async function throwResponseError(res: Response): Promise<never> {
  const body = (await res.json().catch(() => null)) as { error?: unknown; code?: unknown } | null;
  const message = body && typeof body.error === 'string' ? body.error : `request failed (${res.status})`;
  const err = new Error(message) as Error & { code?: string };
  if (body && typeof body.code === 'string') err.code = body.code;
  throw err;
}

async function readJson<T>(res: Response): Promise<T> {
  if (!res.ok) await throwResponseError(res);
  return (await res.json()) as T;
}

export class RealEditorApiClient {
  async listAssets(): Promise<EditorAsset[]> {
    const data = await readJson<{ assets?: EditorAsset[] }>(await fetch('/api/editor/assets', { cache: 'no-store' }));
    return data.assets ?? [];
  }

  async uploadAsset(file: File): Promise<EditorAsset> {
    const form = new FormData();
    form.append('video', file, file.name);
    return readJson<EditorAsset>(await agentAwareFetch('/api/editor/assets', { method: 'POST', body: form }));
  }

  async importAsset(input: {
    source: 'demo' | 'stream';
    job_id: string;
    variant?: string;
    name?: string;
  }): Promise<EditorAsset> {
    return readJson<EditorAsset>(
      await fetch('/api/editor/assets/import', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      }),
    );
  }

  assetMediaUrl(id: string): string {
    return `/api/editor/assets/${id}/media`;
  }

  async listProjects(): Promise<EditorProject[]> {
    const data = await readJson<{ projects?: EditorProject[] }>(await fetch('/api/editor/projects', { cache: 'no-store' }));
    return data.projects ?? [];
  }

  async createProject(title?: string): Promise<EditorProject> {
    return readJson<EditorProject>(
      await fetch('/api/editor/projects', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title }),
      }),
    );
  }

  async getProject(id: string): Promise<EditorProject> {
    return readJson<EditorProject>(await fetch(`/api/editor/projects/${id}`, { cache: 'no-store' }));
  }

  async putPlan(id: string, plan: EditorDocument): Promise<EditorDocument> {
    return readJson<EditorDocument>(
      await fetch(`/api/editor/projects/${id}/plan`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(plan),
      }),
    );
  }

  async preview(id: string, time: number): Promise<EditorSample> {
    return readJson<EditorSample>(
      await fetch(`/api/editor/projects/${id}/preview`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ time }),
      }),
    );
  }

  async startRender(id: string): Promise<EditorRenderState> {
    return readJson<EditorRenderState>(await fetch(`/api/editor/projects/${id}/render`, { method: 'POST' }));
  }

  async getRender(id: string): Promise<EditorRenderState> {
    return readJson<EditorRenderState>(await fetch(`/api/editor/projects/${id}/render`, { cache: 'no-store' }));
  }

  videoUrl(id: string): string {
    return `/api/editor/projects/${id}/render/video`;
  }

  coverUrl(id: string): string {
    return `/api/editor/projects/${id}/render/cover`;
  }
}

export const editorApi = new RealEditorApiClient();
