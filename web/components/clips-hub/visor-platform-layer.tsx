'use client';

import { useId, useState, type ReactNode } from 'react';
import { layerZones, parsePlatformLayer, PLATFORM_LAYER_OPTIONS, type PlatformLayer } from '@/lib/clips/platform-zones';
import { Segmented } from '@/components/ui/segmented';

const LAYER_STORAGE_KEY = 'cliphub-visor-capa';

function readLayer(): PlatformLayer {
  try {
    return parsePlatformLayer(localStorage.getItem(LAYER_STORAGE_KEY));
  } catch {
    return 'todas';
  }
}

function writeLayer(layer: PlatformLayer): void {
  try {
    localStorage.setItem(LAYER_STORAGE_KEY, layer);
  } catch {
    return;
  }
}

export function usePlatformLayer(): [PlatformLayer, (layer: PlatformLayer) => void] {
  const [layer, setLayer] = useState(readLayer);
  const choose = (next: PlatformLayer): void => {
    writeLayer(next);
    setLayer(next);
  };
  return [layer, choose];
}

export function PlatformLayerShapes({ layer }: { layer: PlatformLayer }): ReactNode {
  const hatchId = `visor-capa-${useId().replace(/[^\w-]/g, '')}`;
  const zones = layerZones(layer);
  if (zones.length === 0) return null;
  const path = zones
    .map((zone) => `M${zone.left * 9} ${zone.top * 16}H${zone.right * 9}V${zone.bottom * 16}H${zone.left * 9}Z`)
    .join('');
  return (
    <svg aria-hidden viewBox="0 0 9 16" preserveAspectRatio="none" className="pointer-events-none absolute inset-0 size-full">
      <defs>
        <pattern id={hatchId} width="0.32" height="0.32" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
          <rect width="0.06" height="0.32" fill="rgb(255 255 255 / 0.28)" />
        </pattern>
      </defs>
      <path d={path} fill="rgb(0 0 0 / 0.38)" />
      <path d={path} fill={`url(#${hatchId})`} />
    </svg>
  );
}

const PICKER_OPTIONS = PLATFORM_LAYER_OPTIONS.map(({ value, label, narrowLabel }) => ({
  value,
  label: narrowLabel === undefined ? label : (
    <>
      <span aria-hidden className="md:hidden">{narrowLabel}</span>
      <span className="max-md:sr-only">{label}</span>
    </>
  ),
}));

export function PlatformLayerPicker({ value, onChange }: { value: PlatformLayer; onChange: (layer: PlatformLayer) => void }): ReactNode {
  return <Segmented label="Lo que tapa cada plataforma" value={value} options={PICKER_OPTIONS} onChange={onChange} />;
}
