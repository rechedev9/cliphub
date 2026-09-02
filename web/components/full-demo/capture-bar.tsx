'use client';

import type { CreativeBriefItem } from '@/lib/reel-brief';
import { Button } from '@/components/ui/button';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import {
  canStartFullDemoCapture,
  FULL_DEMO_OVERLAY_THEME_OPTIONS,
} from '@/lib/full-demo';
import { isOverlayTheme, type OverlayTheme } from '@/lib/api/types';

export function FullDemoCaptureBar({
  roundCount,
  emptyHint,
  creating,
  briefItems,
  briefApproved,
  onBriefApprovedChange,
  overlayTheme,
  onOverlayThemeChange,
  onCreate,
}: {
  roundCount: number;
  emptyHint: string;
  creating: boolean;
  briefItems: CreativeBriefItem[];
  briefApproved: boolean;
  onBriefApprovedChange: (approved: boolean) => void;
  overlayTheme: OverlayTheme;
  onOverlayThemeChange: (theme: OverlayTheme) => void;
  onCreate: () => void;
}) {
  const configured = roundCount > 0;
  const ready = canStartFullDemoCapture({ roundCount, briefApproved, creating });

  return (
    <div className="sticky bottom-0 z-20 -mx-(--shell-gutter) mt-2 border-t border-border-accent bg-surface-1 px-(--shell-gutter) py-4 shadow-[0_-12px_28px_-18px_oklch(0.02_0.02_264/0.9)]">
      <div className="flex flex-col gap-4">
        <section className="studio-panel px-4 py-3" aria-labelledby="full-demo-brief-title">
          <p id="full-demo-brief-title" className="font-mono text-meta uppercase tracking-wider text-primary">
            Configuración exacta de captura
          </p>
          <dl className="mt-2.5 grid gap-x-6 gap-y-1.5 text-body-sm @[42rem]/content:grid-cols-2 @[70rem]/content:grid-cols-3">
            {briefItems.map((item) => (
              <div key={item.label} className="flex min-w-0 gap-1.5">
                <dt className="shrink-0 text-fg-3">{item.label}:</dt>
                <dd className="truncate text-fg-1" title={item.value}>{item.value}</dd>
              </div>
            ))}
          </dl>
          <div className="mt-3.5 border-l-2 border-primary bg-primary/8 px-3 py-2.5 text-body-sm text-fg-2">
            <span className="font-mono text-meta uppercase tracking-wider text-primary">FACEIT obligatorio</span>
            <span className="ml-2">ClipHub verifica el perfil y el historial de todos los jugadores antes de abrir HLAE.</span>
          </div>
          <div className="mt-3.5 flex flex-col gap-2">
            <p id="full-demo-theme-label" className="font-mono text-meta uppercase tracking-wider text-fg-3">
              Tema de overlays FACEIT
            </p>
            <ToggleGroup
              type="single"
              variant="filter"
              spacing={2}
              value={overlayTheme}
              onValueChange={(value) => {
                if (isOverlayTheme(value)) onOverlayThemeChange(value);
              }}
              aria-labelledby="full-demo-theme-label"
              className="flex flex-wrap"
            >
              {FULL_DEMO_OVERLAY_THEME_OPTIONS.map((option) => (
                <ToggleGroupItem
                  key={option.value}
                  value={option.value}
                  aria-label={option.label}
                  disabled={creating}
                >
                  {option.label}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
          <label className="mt-3.5 flex min-h-10 items-center gap-2.5 text-body-sm text-fg-1">
            <input
              type="checkbox"
              checked={briefApproved}
              disabled={!configured || creating}
              onChange={(event) => onBriefApprovedChange(event.target.checked)}
              className="size-5 shrink-0 cursor-pointer accent-primary disabled:cursor-not-allowed disabled:opacity-50"
            />
            Confirmo esta configuración para iniciar la captura local de la partida.
          </label>
        </section>

        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3">
          <div className="min-w-0 flex-1">
            <p className="font-mono text-meta uppercase tracking-widest text-fg-3">VÍDEO COMPLETO</p>
            {configured ? (
              <p className="mt-1 truncate font-mono text-body uppercase text-fg-1">
                {roundCount} {roundCount === 1 ? 'ronda' : 'rondas'}
                <span className="text-fg-3"> · </span>
                <span className="text-primary">POV nativo</span>
                <span className="text-fg-3"> · 16:9 · sin música</span>
              </p>
            ) : (
              <p className="mt-1 truncate text-body-sm text-fg-2">{emptyHint}</p>
            )}
          </div>

          <Button
            variant="hero"
            size="lg"
            disabled={!ready}
            loading={creating}
            loadingText="INICIANDO CAPTURA…"
            onClick={onCreate}
            className="neon-notch shrink-0 focus-visible:-outline-offset-4"
          >
            INICIAR CAPTURA
          </Button>
        </div>
      </div>
    </div>
  );
}
