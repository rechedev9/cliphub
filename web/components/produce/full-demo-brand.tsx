'use client';

import { useId, type ReactNode } from 'react';
import { BRAND_CORNER_LABELS, bannerWindowLabel, brandBannerOf, brandSubscribeOf, brandWatermarkOf, type BrandBanner, type BrandOverlay, type BrandWatermark } from '@/lib/brand-kit';
import type { FullDemoOptions } from '@/lib/full-demo-plan';
import { Button } from '@/components/ui/button';
import { BrandKitPanel, useBrandKit } from '@/components/clips/brand-kit-editor';
import { FullDemoGroup } from './full-demo-fields';

/** Copies the saved brand kit into the plan: the logo watermark and the sponsor banner. */
export function FullDemoBrand({ options, onChange }: { options: FullDemoOptions; onChange: (options: FullDemoOptions) => void }): ReactNode {
  const { kit, error, setKit } = useBrandKit();
  const id = useId();
  const brand = options.overlays.brand ?? {};
  const kitWatermark = kit ? brandWatermarkOf(kit) : undefined;
  const kitBanner = kit ? brandBannerOf(kit) : undefined;
  const kitSubscribe = kit ? brandSubscribeOf(kit) : undefined;
  const stale = (brand.watermark !== undefined && JSON.stringify(brand.watermark) !== JSON.stringify(kitWatermark))
    || (brand.banner !== undefined && JSON.stringify(brand.banner) !== JSON.stringify(kitBanner))
    || (options.overlays.subscribe === true && JSON.stringify(brand.subscribe) !== JSON.stringify(kitSubscribe));

  function setBrand(next: BrandOverlay): void {
    const { brand: _brand, ...overlays } = options.overlays;
    onChange({ ...options, overlays: Object.keys(next).length > 0 ? { ...overlays, brand: next } : overlays });
  }

  function toggle(part: 'watermark' | 'banner', on: boolean): void {
    const { [part]: _part, ...rest } = brand;
    const copy = part === 'watermark' ? kitWatermark : kitBanner;
    setBrand(on && copy ? { ...rest, [part]: copy } : rest);
  }

  function refresh(): void {
    setBrand({
      ...(brand.watermark && kitWatermark ? { watermark: kitWatermark } : {}),
      ...(brand.banner && kitBanner ? { banner: kitBanner } : {}),
      ...(options.overlays.subscribe === true && kitSubscribe ? { subscribe: kitSubscribe } : {}),
    });
  }

  const watermarkDetail = watermarkText(brand.watermark, kitWatermark !== undefined);
  const bannerDetail = bannerText(brand.banner, kitBanner !== undefined);

  return <FullDemoGroup title="Kit de marca" note="Tu logo como marca de agua y tu banner de sponsor sobre las rondas. Tus clips de intro, sponsor y cierre quedan como los subiste.">
    <div className="grid min-w-0 gap-3">
      <BrandCheck id={`${id}-watermark`} title="Marca de agua" detail={watermarkDetail} checked={brand.watermark !== undefined}
        disabled={!brand.watermark && !kitWatermark} onChange={(on) => toggle('watermark', on)} />
      <BrandCheck id={`${id}-banner`} title="Banner de sponsor" detail={bannerDetail} checked={brand.banner !== undefined}
        disabled={!brand.banner && !kitBanner} onChange={(on) => toggle('banner', on)} />
      {kit && stale ? <div className="flex flex-wrap items-center gap-3">
        <p className="text-body-sm text-fg-2">El plan guarda una copia anterior del kit.</p>
        <Button type="button" variant="secondary" onClick={refresh}>Usar el kit guardado</Button>
      </div> : null}
      <BrandKitPanel kit={kit} error={error} onSaved={setKit} />
    </div>
  </FullDemoGroup>;
}

function watermarkText(copy: BrandWatermark | undefined, saved: boolean): string {
  if (copy) return `En la esquina ${BRAND_CORNER_LABELS[copy.corner].toLowerCase()} al ${Math.round(copy.opacity * 100)} %.`;
  return saved ? 'Tu logo en una esquina de cada ronda.' : 'Sube el logo de tu canal en el kit para usarla.';
}

function bannerText(copy: BrandBanner | undefined, saved: boolean): string {
  if (copy) return `${bannerWindowLabel(copy.start_seconds, copy.end_seconds)} del vídeo, arriba, bajo el marcador.`;
  return saved ? 'Arriba, bajo el marcador, en la ventana de tiempo del kit.' : 'Sube tu banner de sponsor en el kit para usarlo.';
}

function BrandCheck({ id, title, detail, checked, disabled, onChange }: {
  id: string; title: string; detail: string; checked: boolean; disabled: boolean; onChange: (on: boolean) => void;
}): ReactNode {
  return <label className="flex min-w-0 cursor-pointer items-start gap-3 text-body-sm has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60">
    <input type="checkbox" checked={checked} disabled={disabled} aria-label={title} aria-describedby={id}
      onChange={(event) => onChange(event.target.checked)} className="mt-0.5 size-4 shrink-0 accent-format-full" />
    <span className="min-w-0">
      <span className="block font-semibold text-fg-1">{title}</span>
      <span id={id} className="mt-0.5 block text-fg-2">{detail}</span>
    </span>
  </label>;
}
