'use client';

import { useEffect, useState, type ReactElement } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Layers } from 'lucide-react';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { editorApi, EDITOR_STATUS, type EditorProject } from '@/lib/api/editor';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';

export function EditorHome(): ReactElement {
  const router = useRouter();
  const [projects, setProjects] = useState<EditorProject[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    editorApi
      .listProjects()
      .then((list) => {
        if (!cancelled) setProjects(list);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const code = err instanceof Error ? (err as Error & { code?: string }).code : undefined;
        setError(code === SERVICE_UNAVAILABLE_CODE ? 'El orquestador no está en marcha.' : 'No se pudieron cargar los proyectos.');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  async function createProject(): Promise<void> {
    setBusy(true);
    try {
      const project = await editorApi.createProject('Nuevo montaje');
      router.push(`/editor/${project.id}`);
    } catch {
      setError('No se pudo crear el proyecto.');
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="Editor"
        description="Monta los MP4 ya renderizados en un timeline multitrack. El plan persistido es canónico; FFmpeg renderiza el final."
        actions={
          <Button onClick={() => void createProject()} disabled={busy}>
            Nuevo proyecto
          </Button>
        }
      />
      {error !== null ? <p className="text-body-sm text-destructive">{error}</p> : null}
      {projects.length === 0 && error === null ? (
        <StudioEmptyState
          icon={Layers}
          title="Todavía no hay montajes"
          description="Crea un proyecto, sube MP4s de la biblioteca o de un render, y arma el timeline."
        />
      ) : (
        <ul className="grid gap-3">
          {projects.map((project) => (
            <li key={project.id}>
              <Link
                href={`/editor/${project.id}`}
                className="flex items-center justify-between rounded-lg border border-border bg-surface-2 px-4 py-3 hover:bg-surface-3"
              >
                <span className="font-medium text-fg-1">{project.title}</span>
                <span className="text-caption text-fg-3">
                  {project.status === EDITOR_STATUS.rendered ? 'Renderizado' : project.status}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
