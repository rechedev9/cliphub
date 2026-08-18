'use client';

import { useEffect, useState, type ReactElement } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ChevronRight, Layers } from 'lucide-react';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { StudioPageHeader } from '@/components/studio/page-header';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { editorApi, EDITOR_STATUS, type EditorProject, type EditorStatus } from '@/lib/api/editor';
import { MUTATION_CAPABILITY_ERROR } from '@/lib/api/local-request-guard';
import { SERVICE_UNAVAILABLE_CODE } from '@/lib/api/types';
import { cn } from '@/lib/utils';

export function EditorHome(): ReactElement {
  const router = useRouter();
  const [projects, setProjects] = useState<EditorProject[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    editorApi
      .listProjects()
      .then((list) => {
        if (cancelled) return;
        setProjects(list);
        setError(null);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(editorHomeError(err, 'No se pudieron cargar los proyectos.'));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
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
    } catch (err: unknown) {
      setError(editorHomeError(err, 'No se pudo crear el proyecto.'));
      setBusy(false);
    }
  }

  const createButton = (
    <Button onClick={() => void createProject()} disabled={busy} loading={busy}>
      Nuevo proyecto
    </Button>
  );

  return (
    <div className="flex flex-col gap-8">
      <StudioPageHeader
        title="Editor"
        description="Monta los MP4 ya renderizados en un timeline multitrack. El plan persistido es canónico; FFmpeg renderiza el final."
        actions={
          <Button onClick={() => void createProject()} disabled={busy} loading={busy}>
            Nuevo proyecto
          </Button>
        }
      />
      {error !== null ? (
        <p role="alert" className="studio-panel border-destructive/45 px-4 py-3 text-body-sm text-destructive">
          {error}
        </p>
      ) : null}
      <EditorHomeBody loading={loading} error={error} projects={projects} createButton={createButton} />
    </div>
  );
}

function EditorHomeBody({
  loading,
  error,
  projects,
  createButton,
}: {
  loading: boolean;
  error: string | null;
  projects: EditorProject[];
  createButton: ReactElement;
}): ReactElement | null {
  if (loading) return <EditorHomeSkeleton />;
  if (projects.length === 0 && error === null) {
    return (
      <StudioEmptyState
        icon={Layers}
        title="Todavía no hay montajes"
        description="Crea un proyecto, sube MP4s de la biblioteca o de un render, y arma el timeline."
        actions={createButton}
      />
    );
  }
  if (projects.length === 0) return null;
  return (
    <ul className="grid gap-3">
      {projects.map((project) => {
        const status = projectStatusPresentation(project.status);
        return (
          <li key={project.id}>
            <Link
              href={`/editor/${project.id}`}
              className={cn(
                'studio-panel studio-panel-interactive flex min-h-[72px] items-center justify-between gap-4 px-4 py-4 sm:px-5',
                FOCUS_RING,
              )}
            >
              <span className="min-w-0 truncate font-display text-body font-bold uppercase tracking-tight text-fg-1">
                {project.title}
              </span>
              <span className="flex shrink-0 items-center gap-3">
                <StatusTag tone={status.tone} dot>
                  {status.label}
                </StatusTag>
                <ChevronRight className="size-4 text-fg-3" aria-hidden />
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

function EditorHomeSkeleton(): ReactElement {
  return (
    <ul role="status" aria-busy="true" aria-label="Cargando proyectos" className="grid gap-3">
      {Array.from({ length: 3 }, (_, i) => (
        <li key={i} className="studio-panel flex min-h-[72px] items-center justify-between gap-4 px-4 py-4 sm:px-5">
          <Skeleton className="h-5 w-40 max-w-[50%]" />
          <Skeleton className="h-7 w-24" />
        </li>
      ))}
    </ul>
  );
}

function projectStatusPresentation(status: EditorStatus): { label: string; tone: StatusTagTone } {
  switch (status) {
    case EDITOR_STATUS.draft:
      return { label: 'Borrador', tone: 'neutral' };
    case EDITOR_STATUS.rendering:
      return { label: 'Renderizando', tone: 'primary' };
    case EDITOR_STATUS.rendered:
      return { label: 'Renderizado', tone: 'success' };
    case EDITOR_STATUS.failed:
      return { label: 'Fallido', tone: 'danger' };
  }
}

function editorHomeError(err: unknown, fallback: string): string {
  if (isServiceUnavailable(err)) return 'El orquestador no está en marcha.';
  if (isCapabilityDenied(err)) {
    return 'Falta la capacidad local. Abre Studio desde Local Studio o autoriza en /bootstrap.';
  }
  return fallback;
}

function isServiceUnavailable(err: unknown): boolean {
  return typeof err === 'object' && err !== null && 'code' in err && err.code === SERVICE_UNAVAILABLE_CODE;
}

function isCapabilityDenied(err: unknown): boolean {
  return err instanceof Error && err.message === MUTATION_CAPABILITY_ERROR;
}
