'use client';

import { useCallback, useEffect, useState } from 'react';
import {
  CircleCheck,
  RefreshCw,
  Settings2,
  TriangleAlert,
  WifiOff,
  type LucideIcon,
} from 'lucide-react';
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
import { cn } from '@/lib/utils';

/** `bar` is the 2px status edge down the rail-native card's left side. */
const STATUS_META: Record<
  CaptureStatus,
  { label: string; text: string; bar: string; hint: string; icon: LucideIcon }
> = {
  ready: {
    label: 'Lista',
    text: 'text-success',
    bar: 'bg-success',
    hint: 'HLAE + CS2 detectados en este PC',
    icon: CircleCheck,
  },
  warning: {
    label: 'Revisa rutas',
    text: 'text-warning',
    bar: 'bg-warning',
    hint: 'Una herramienta configurada no está disponible.',
    icon: TriangleAlert,
  },
  unconfigured: {
    label: 'Configurar',
    text: 'text-destructive',
    bar: 'bg-destructive',
    hint: 'No se encontró HLAE + CS2 en este PC.',
    icon: Settings2,
  },
  offline: {
    label: 'Sin conexión',
    text: 'text-fg-3',
    bar: 'bg-fg-4',
    hint: 'Arranca el servicio local de TickCut.',
    icon: WifiOff,
  },
};

/** The three record tools, with a friendly name and a typical Windows path. */
const TOOL_GUIDE: Array<{ name: string; label: string; example: string }> = [
  { name: 'ZV_RECORDER_PATH', label: 'Grabador TickCut', example: 'C:\\...\\bin\\zv-recorder.exe' },
  { name: 'ZV_HLAE_PATH', label: 'HLAE', example: 'C:\\HLAE-<latest-version>\\HLAE.exe' },
  { name: 'ZV_CS2_PATH', label: 'CS2', example: 'C:\\Program Files (x86)\\Steam\\steamapps\\common\\Counter-Strike Global Offensive\\game\\bin\\win64\\cs2.exe' },
];

/**
 * CaptureReadiness — the CAPTURA card pinned to the sidebar footer: a
 * bracket-cornered HUD panel that always tells the user whether gameplay
 * capture is set up on their machine, reading the local orchestrator's
 * /api/capabilities. Clicking it opens a dialog with the exact env vars +
 * example paths to set, each tool's live accessible state, and the restart
 * note. This is the always-on reminder so capture never silently fails.
 */
export function CaptureReadiness() {
  const [data, setData] = useState<CaptureReadinessData | null>(null);
  const [loading, setLoading] = useState(false);

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
  const StatusIcon = meta.icon;
  const toolState = new Map((data?.tools ?? []).map((t) => [t.name, t]));

  return (
    <Dialog>
      <DialogTrigger asChild>
        {/*
          Rail-native, not a content card. `.studio-panel` carries a 20px
          ambient drop designed for a large surface, and
          `.studio-panel-interactive` adds a hover lift — so the sidebar's only
          status object was a card levitating inside a navigation rail. It is
          now full-bleed to the wall with a 2px status edge and a surface-only
          hover, which is what everything else in the rail speaks.
        */}
        <button
          type="button"
          aria-label={`Captura: ${meta.label}`}
          title={`Captura: ${meta.label}`}
          className="relative w-full border-y border-sidebar-border bg-surface-1 py-3 pr-3 pl-4 text-left transition-colors duration-(--dur-fast) ease-standard hover:bg-surface-2 group-data-[collapsible=icon]:mx-auto group-data-[collapsible=icon]:grid group-data-[collapsible=icon]:size-10 group-data-[collapsible=icon]:place-items-center group-data-[collapsible=icon]:border-y-0 group-data-[collapsible=icon]:bg-transparent group-data-[collapsible=icon]:p-0"
        >
          <span
            aria-hidden
            className={cn(
              'absolute inset-y-0 left-0 w-[2px] group-data-[collapsible=icon]:hidden',
              meta.bar,
            )}
          />
          <div className="group-data-[collapsible=icon]:hidden">
            <div className="flex items-center justify-between gap-3 font-mono text-meta uppercase tracking-wider">
              <span className="text-fg-3">Captura</span>
              <span className={cn('inline-flex items-center gap-1.5', meta.text)}>
                <StatusIcon className="size-3.5" aria-hidden />
                {meta.label}
              </span>
            </div>
            <p className="mt-2 text-body-sm text-fg-2">{meta.hint}</p>
          </div>
          <span className={cn('hidden group-data-[collapsible=icon]:block', meta.text)}>
            <StatusIcon className="size-4" aria-hidden />
          </span>
        </button>
      </DialogTrigger>

      <DialogContent>
        <DialogHeader>
          <DialogTitle>Captura de gameplay</DialogTitle>
          <DialogDescription>
            TickCut graba en tu PC con HLAE + CS2 y los encuentra automáticamente. Esto es lo que detectó en esta máquina:
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          {TOOL_GUIDE.map((tool) => {
            const state = toolState.get(tool.name);
            const found = Boolean(state?.accessible);
            let badge: { label: string; cls: string };
            if (found) {
              badge = { label: state?.source === 'env' ? 'Configurada' : 'Detectada', cls: 'bg-primary/10 text-primary' };
            } else if (state?.configured) {
              badge = { label: 'Falta', cls: 'bg-amber-400/10 text-amber-400' };
            } else {
              badge = { label: 'No encontrada', cls: 'bg-destructive/10 text-destructive' };
            }
            return (
              <div key={tool.name} className="studio-panel p-4">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[15px] font-medium text-foreground">{tool.label}</span>
                  <span className={cn('shrink-0 px-1.5 py-0.5 font-[family-name:var(--font-mono)] text-[0.6rem] font-semibold uppercase tracking-wider', badge.cls)}>
                    {badge.label}
                  </span>
                </div>
                {found ? (
                  <p className="mt-1 text-sm text-muted-foreground">Lista para usar.</p>
                ) : (
                  <>
                    <p className="mt-1 text-xs text-muted-foreground">
                      No encontrada. Instálala, o apunta <code className="font-[family-name:var(--font-mono)]">{tool.name}</code> a su ruta.
                    </p>
                    <p className="mt-1 break-all font-[family-name:var(--font-mono)] text-[0.7rem] text-muted-foreground/70">normalmente {tool.example}</p>
                  </>
                )}
              </div>
            );
          })}
        </div>

        <p className="text-xs text-muted-foreground">
          ¿Instalada en el sitio habitual? Se detecta automáticamente. Para usar una ruta propia, define la variable de entorno de
          arriba y reinicia el orquestador (vuelve a lanzar <code className="font-[family-name:var(--font-mono)]">zv serve</code>); la
          configuración del worker se lee al arrancar.
        </p>

        <div className="flex justify-end">
          <Button variant="secondary" size="sm" onClick={() => void refresh()} disabled={loading}>
            <RefreshCw className={cn('size-4', loading && 'animate-spin')} />
            Volver a comprobar
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
