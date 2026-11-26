'use client';

import { BadgeCheck, BellRing, Megaphone } from 'lucide-react';
import type { ReactNode } from 'react';
import { BRAND_CORNER_LABELS, bannerWindowLabel, type BrandKit } from '@/lib/brand-kit';
import type { EditConfig } from '@/lib/api/types';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { BrandKitPanel, useBrandKit } from './brand-kit-editor';

/** The Short's brand switches and the editor of the saved kit they copy. */
export function BrandKitOptions({ value, onChange, disabled = false }: { value: EditConfig; onChange: (next: EditConfig) => void; disabled?: boolean }): ReactNode {
  const { kit, error, setKit } = useBrandKit();
  const watermark = kit?.logo
    ? `Tu logo en la esquina ${BRAND_CORNER_LABELS[kit.watermark_corner].toLowerCase()} al ${Math.round(kit.watermark_opacity * 100)} %.`
    : 'Sube el logo de tu canal en el kit para usar la marca de agua.';
  const banner = kit?.banner
    ? `${bannerWindowLabel(kit.banner_start_seconds, kit.banner_end_seconds)}, donde va el banner afiliado, que se quita.`
    : 'Sube tu banner de sponsor en el kit para usarlo.';
  const subscribe = subscribeText(kit);
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <ToggleGroup
        type="multiple"
        value={[value.brandWatermark ? 'brand-watermark' : '', value.brandBanner ? 'brand-banner' : '', value.subscribeBanner ? 'subscribe-banner' : ''].filter(Boolean)}
        onValueChange={(items) => onChange({ ...value, brandWatermark: items.includes('brand-watermark'), brandBanner: items.includes('brand-banner'), subscribeBanner: items.includes('subscribe-banner') })}
        disabled={disabled}
        variant="outline"
        className="flex-wrap"
      >
        <ToggleGroupItem value="brand-watermark" aria-label="Marca de agua" disabled={disabled || (!kit?.logo && !value.brandWatermark)}>
          <BadgeCheck className="size-4" />
          Marca de agua
        </ToggleGroupItem>
        <ToggleGroupItem value="brand-banner" aria-label="Banner de sponsor" disabled={disabled || (!kit?.banner && !value.brandBanner)}>
          <Megaphone className="size-4" />
          Banner de sponsor
        </ToggleGroupItem>
        <ToggleGroupItem value="subscribe-banner" aria-label="Suscríbete" disabled={disabled}>
          <BellRing className="size-4" />
          Suscríbete
        </ToggleGroupItem>
      </ToggleGroup>
      {kit ? (
        <>
          <p className="text-body-sm text-fg-3">{watermark} {banner} {subscribe}</p>
          <p className="text-body-sm text-fg-3">El vídeo usa el kit tal como esté al crearlo: cambiarlo después no altera los vídeos ya hechos.</p>
        </>
      ) : null}
      <BrandKitPanel kit={kit} error={error} onSaved={setKit} disabled={disabled} />
    </div>
  );
}

function subscribeText(kit: BrandKit | null): string {
  const where = 'Suscríbete sale una vez, hacia un tercio del Short';
  if (kit?.logo && kit.channel_name) return `${where}, con tu logo y «${kit.channel_name}».`;
  if (kit?.channel_name) return `${where}, con «${kit.channel_name}». Sube tu logo en el kit para que salga también.`;
  if (kit?.logo) return `${where}, con tu logo. Añade el nombre del canal en el kit para que salga también.`;
  return `${where}. Añade tu logo y el nombre del canal en el kit para personalizarlo.`;
}
