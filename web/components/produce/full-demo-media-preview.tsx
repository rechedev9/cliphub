'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { FullDemoAssetRef } from '@/lib/full-demo-plan';

export function FullDemoMediaPreview({ asset, video = false, label, gain = 1 }: {
  asset: FullDemoAssetRef; video?: boolean; label: string; gain?: number;
}): ReactNode {
  const player = useRef<HTMLMediaElement | null>(null);
  const [failedSource, setFailedSource] = useState<string | null>(null);
  const source = `/api/editor/assets/${asset.id}/media`;
  useEffect(() => { if (player.current) player.current.volume = Math.min(1, Math.max(0, gain)); }, [gain, source]);
  const props = { controls: true, preload: 'none', src: source, 'aria-label': label, onError: () => setFailedSource(source) };
  return <div className="w-full space-y-1.5">
    {video ? <video ref={(element) => { player.current = element; }} {...props} playsInline className="aspect-video w-full rounded-sm bg-black" /> : <audio ref={(element) => { player.current = element; }} {...props} className="w-full" />}
    <p className="text-meta text-fg-3">Previsualización del archivo original. La normalización y la mezcla se verifican al renderizar.</p>
    {failedSource === source ? <p role="alert" className="text-body-sm text-destructive">No se pudo reproducir este archivo. Comprueba que Studio está conectado o usa otro formato compatible.</p> : null}
  </div>;
}
