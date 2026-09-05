'use client';

import { useCallback, useEffect, useState, useSyncExternalStore, type ReactElement } from 'react';
import { MonitorPlay, RefreshCw } from 'lucide-react';
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
import { Button, FOCUS_RING } from '@/components/ui/button';
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
const TOOL_GUIDE: Array<{ name: string; label: string; help: string; example: string }> = [
  { name: 'ZV_RECORDER_PATH', label: 'Grabador ClipHub', help: 'Viene incluido en ClipHub Studio para Windows. Si falta, reinstala la aplicación.', example: 'C:\\...\\bin\\zv-recorder.exe' },
  { name: 'ZV_HLAE_PATH', label: 'HLAE', help: 'Es la herramienta que permite a ClipHub grabar CS2. Debe estar instalada en el mismo PC.', example: 'C:\\HLAE-<latest-version>\\HLAE.exe' },
  { name: 'ZV_CS2_PATH', label: 'CS2', help: 'Instala Counter-Strike 2 desde Steam en este PC y vuelve a abrir ClipHub.', example: 'C:\\Program Files (x86)\\Steam\\steamapps\\common\\Counter-Strike Global Offensive\\game\\bin\\win64\\cs2.exe' },
];

/**
 * Sidebar footer status pill: capture readiness from the local orchestrator,
 * overridden by the live REC job while CS2 + HLAE record. Click opens the setup dialog.
 */
export function CaptureReadiness({ variant = 'sidebar' }: { variant?: 'sidebar' | 'settings' }): ReactElement {
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

  let pillText = data === null ? 'Comprobando captura…' : meta.text;
  let label = data === null ? 'Comprobando' : meta.label;
  let pillTone = data === null ? 'border-border text-fg-3' : meta.tone;
  let pillDot = data === null ? 'bg-fg-3' : meta.dot;
  if (recording !== undefined) {
    const detail =
      recording.progress === null
        ? truncate(recording.title, REC_TITLE_MAX)
        : `R${recording.progress.done}/${recording.progress.total}`;
    pillText = `REC · ${detail}`;
    label = `Grabando ${recording.title}`;
    pillTone = REC_TONE;
    pillDot = 'neon-pulse bg-stream';
  }

  return (
    <Dialog>
      <DialogTrigger asChild>
        {variant === 'settings' ? (
          <button type="button" className={`studio-panel studio-panel-interactive flex w-full flex-col gap-3 p-5 text-left ${FOCUS_RING}`}>
            <span className="flex items-center gap-3 font-display text-title font-semibold text-fg-1"><MonitorPlay aria-hidden className="size-5 text-primary" />Grabación de demos</span>
            <span className="text-body-sm text-fg-2">Revisa si este PC tiene CS2, HLAE y el grabador. Son necesarios para crear shorts y vídeos largos desde una demo.</span>
            <span className="flex flex-wrap items-center justify-between gap-3 text-body-sm">
              <span className="text-fg-2">{pillText}</span>
              <span className="font-semibold text-primary">Revisar requisitos →</span>
            </span>
          </button>
        ) : (
        <button
          type="button"
          aria-label={`Captura: ${label}`}
          title={`Captura: ${label}`}
          className={cn(
            'mx-4 flex min-h-10 items-center gap-2 rounded-md border bg-surface-1 px-3 py-2.5 text-left font-[family-name:var(--font-mono)] text-meta tracking-wider uppercase',
            'transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-2',
            'group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:size-10 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:border-0 group-data-[collapsible=icon]:bg-transparent group-data-[collapsible=icon]:p-0',
            pillTone,
          )}
        >
          <span
            aria-hidden
            className={cn(
              'size-2 shrink-0 rounded-full',
              pillDot,
            )}
          />
          <span className="min-w-0 truncate group-data-[collapsible=icon]:hidden">{pillText}</span>
        </button>
        )}
      </DialogTrigger>

      <DialogContent className="max-h-[85dvh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Requisitos de grabación</DialogTitle>
          <DialogDescription>
            Para grabar una demo necesitas ClipHub Studio en Windows, CS2 y HLAE en el mismo PC. Puedes cargar demos y preparar el vídeo antes de tener todo listo.
          </DialogDescription>
        </DialogHeader>

        {status === 'offline' && data !== null ? (
          <p role="status" className="text-body-sm text-warning">No se pudo comprobar este PC. Abre ClipHub Studio y pulsa «Volver a comprobar».</p>
        ) : null}
        <div className="flex flex-col gap-3">
          {TOOL_GUIDE.map((tool) => {
            const state = toolState.get(tool.name);
            const found = Boolean(state?.accessible);
            const badge = toolBadge(found, Boolean(state?.configured), state?.source === 'env');
            return (
              <div key={tool.name} className="studio-panel flex flex-col gap-2 p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <span className="font-display text-body font-bold uppercase text-fg-1">{tool.label}</span>
                  <StatusTag tone={badge.tone}>{badge.label}</StatusTag>
                </div>
                {found ? (
                  <p className="text-body-sm text-fg-2">Lista para usar.</p>
                ) : (
                  <>
                    <p className="text-body-sm text-fg-2">{tool.help}</p>
                  </>
                )}
              </div>
            );
          })}
        </div>

        <p className="text-body-sm text-fg-2">Los clips de stream parten de un vídeo ya grabado y no necesitan CS2 ni HLAE. La conexión de Steam en Ajustes es opcional si ya tienes el archivo de la demo.</p>
        <details className="group/setup border-t border-border-subtle pt-2 text-body-sm text-fg-2">
          <summary className={`min-h-10 cursor-pointer py-2 ${FOCUS_RING}`}>Rutas personalizadas · configuración avanzada</summary>
          <p className="py-2">Si una herramienta está en otra carpeta, configura su variable de entorno y reinicia el servicio local.</p>
          <dl className="flex flex-col gap-3">
            {TOOL_GUIDE.map(tool => <div key={tool.name}><dt className="font-mono text-meta text-fg-1">{tool.name}</dt><dd className="break-all text-body-sm">{tool.example}</dd></div>)}
          </dl>
        </details>

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
