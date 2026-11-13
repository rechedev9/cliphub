'use client';

import { useState, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { MonitorPlay, Smartphone } from 'lucide-react';
import type { LibraryDemo } from '@/lib/api/library';
import { PRODUCE_FORMAT, flowHref, type ProduceFormat } from '@/lib/clips/routes';
import { withoutDemo, withoutDemoReason } from '@/lib/library/partidas';
import { crearJobFor } from '@/components/matches/open-in-crear-button';
import { currentEdition } from '@/lib/edition';
import { Button } from '@/components/ui/button';

/** "Crear Short" and "Demo completa" for a partida: the analysis area hands the demo to Crear. */
export function CreateFromDemo({ demo }: { demo: LibraryDemo }): ReactNode {
  const router = useRouter();
  const [busy, setBusy] = useState<ProduceFormat | null>(null);
  const [error, setError] = useState(false);

  async function open(format: ProduceFormat): Promise<void> {
    setBusy(format);
    setError(false);
    try {
      router.push(flowHref(format, { job: await crearJobFor({ demo, format }) }));
    } catch {
      setError(true);
      setBusy(null);
    }
  }

  if (currentEdition() === 'web') return null;

  if (withoutDemo(demo.source)) {
    return <span className="max-w-sm text-meta text-fg-3">{withoutDemoReason(demo.source)}</span>;
  }

  return (
    <>
      <Button type="button" variant="outline" size="sm" disabled={busy !== null} onClick={() => void open(PRODUCE_FORMAT.short)}>
        <Smartphone aria-hidden />
        {busy === PRODUCE_FORMAT.short ? 'Preparando' : 'Crear Short'}
      </Button>
      <Button type="button" variant="outline" size="sm" disabled={busy !== null} onClick={() => void open(PRODUCE_FORMAT.full)}>
        <MonitorPlay aria-hidden />
        {busy === PRODUCE_FORMAT.full ? 'Preparando' : 'Demo completa'}
      </Button>
      {error ? (
        <span role="alert" className="text-meta text-destructive">
          No se pudo pasar la demo a Crear.
        </span>
      ) : null}
    </>
  );
}
