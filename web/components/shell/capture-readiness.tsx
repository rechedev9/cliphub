'use client';

import { useCallback, useEffect, useState, useSyncExternalStore, type ReactElement } from 'react';
import { RefreshCw } from 'lucide-react';
import { api } from '@/lib/api';
import type { CaptureReadiness as CaptureReadinessData, CaptureStatus } from '@/lib/api/types';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { StatusTag, type StatusTagTone } from '@/components/studio/status-tag';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';
import { cn } from '@/lib/utils';

/** `label` is the short aria name; `text` is the one-line pill copy. */
const STATUS_META: Record<
  CaptureStatus,
  { label: string; text: string; tone: string; dot: string }
> = {
  ready: {
    label: 'Lista',
    text: 'CS2 + HLAE listos',
    tone: 'border-success/45 text-success',
    dot: 'bg-success',
  },
  warning: {
    label: 'Revisa rutas',
    text: 'Revisa rutas',
    tone: 'border-warning/45 text-warning',
    dot: 'bg-warning',
  },
  unconfigured: {
    label: 'Configurar',
    text: 'Configurar',
    tone: 'border-destructive/45 text-destructive',
    dot: 'bg-destructive',
  },
  offline: {
    label: 'Sin conexión',
    text: 'Servicio local offline',
    tone: 'border-destructive/45 text-destructive',
    dot: 'bg-destructive',
  },
};

const REC_TONE = 'border-stream/45 text-stream-text';
const REC_TITLE_MAX = 18;

/** The three record tools, with a friendly name and a typical Windows path. */
const TOOL_GUIDE: Array<{ name: string; label: string; example: string }> = [
  { name: 'ZV_RECORDER_PATH', label: 'Grabador ClipHub', example: 'C:\\...\\bin\\zv-recorder.exe' },
  { name: 'ZV_HLAE_PATH', label: 'HLAE', example: 'C:\\HLAE-<latest-version>\\HLAE.exe' },
  { name: 'ZV_CS2_PATH', label: 'CS2', example: 'C:\\Program Files (x86)\\Steam\\steamapps\\common\\Counter-Strike Global Offensive\\game\\bin\\win64\\cs2.exe' },
];

/**
 * Sidebar footer status pill: capture readiness from the local orchestrator,
 * overridden by the live REC job while CS2 + HLAE record. Click opens the setup dialog.
 */
export function CaptureReadiness(): ReactElement {
  const [data, setData] = useState<CaptureReadinessData | null>(null);
  const [loading, setLoading] = useState(false);
  const activity = useSyncExternalStore(
    subscribeToShellActivity,
    shellActivitySnapshot,
    serverShellActivitySnapshot,
  );

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setData(await api.getCaptureReadiness());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const status: CaptureStatus = data?.status ?? 'offline';
  const meta = STATUS_META[status];
  const recording = activity.jobs.find((job) => job.stage === 'recording');
  const toolState = new Map((data?.tools ?? []).map((t) => [t.name, t]));

  let pillText = meta.text;
  let label = meta.label;
  if (recording !== undefined) {
    const detail =
      recording.progress === null
        ? truncate(recording.title, REC_TITLE_MAX)
        : `R${recording.progress.done}/${recording.progress.total}`;
    pillText = `REC · ${detail}`;
    label = `Grabando ${recording.title}`;
  }

  return (
    <Dialog>
      <DialogTrigger asChild>
        <button
          type="button"
          aria-label={`Captura: ${label}`}
          title={`Captura: ${label}`}
          className={cn(
            'mx-4 flex min-h-10 items-center gap-2 rounded-md border bg-surface-1 px-3 py-2.5 text-left font-[family-name:var(--font-mono)] text-meta tracking-wider uppercase',
            'transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-2',
            'group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:size-10 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:border-0 group-data-[collapsible=icon]:bg-transparent group-data-[collapsible=icon]:p-0',
            recording === undefined ? meta.tone : REC_TONE,
          )}
        >
          <span
            aria-hidden
            className={cn(
              'size-2 shrink-0 rounded-full',
              recording === undefined ? meta.dot : 'neon-pulse bg-stream',
            )}
          />
          <span className="min-w-0 truncate group-data-[collapsible=icon]:hidden">{pillText}</span>
        </button>
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>Captura de gameplay</DialogTitle>
          <DialogDescription>
            ClipHub graba en tu PC con HLAE + CS2 y los encuentra automáticamente. Esto es lo que detectó en esta máquina:
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          {TOOL_GUIDE.map((tool) => {
            const state = toolState.get(tool.name);
            const found = Boolean(state?.accessible);
            const badge = toolBadge(found, Boolean(state?.configured), state?.source === 'env');
            return (
              <div key={tool.name} className="studio-panel flex flex-col gap-2 p-4">
                <div className="flex items-center justify-between gap-3">
                  <span className="min-w-0 truncate font-display text-body font-bold uppercase text-fg-1">{tool.label}</span>
                  <StatusTag tone={badge.tone}>{badge.label}</StatusTag>
                </div>
                {found ? (
                  <p className="text-body-sm text-fg-2">Lista para usar.</p>
                ) : (
                  <>
                    <p className="text-body-sm text-fg-2">
                      No encontrada. Instálala, o apunta <code className="font-mono text-fg-1">{tool.name}</code> a su ruta.
                    </p>
                    <p className="break-all font-mono text-meta tracking-wider text-fg-3">normalmente {tool.example}</p>
                  </>
                )}
              </div>
            );
          })}
        </div>

        <p className="text-body-sm text-fg-2">
          ¿Instalada en el sitio habitual? Se detecta automáticamente. Para usar una ruta propia, define la variable de entorno de
          arriba y reinicia el orquestador (vuelve a lanzar <code className="font-mono text-fg-1">zv serve</code>); la
          configuración del worker se lee al arrancar.
        </p>

        <div className="flex justify-end">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void refresh()}
            loading={loading}
            loadingText="COMPROBANDO…"
          >
            <RefreshCw className="size-4" />
            Volver a comprobar
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function truncate(text: string, max: number): string {
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}

/** Detected / configured-but-missing / absent, as one of the kit's tag tones. */
function toolBadge(found: boolean, configured: boolean, fromEnv: boolean): { label: string; tone: StatusTagTone } {
  if (found) return { label: fromEnv ? 'Configurada' : 'Detectada', tone: 'primary' };
  return configured ? { label: 'Falta', tone: 'warning' } : { label: 'No encontrada', tone: 'danger' };
}
