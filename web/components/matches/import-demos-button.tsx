'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Upload } from 'lucide-react';
import { currentEdition } from '@/lib/edition';
import { DemoImport } from '@/components/matches/demo-import';
import { Button } from '@/components/ui/button';

const noop = (): void => undefined;

/**
 * Importar demos where a coach preps a rival: the web edition opens its bulk import page (the browser reads
 * the .dem and .dem.zst files), the desktop app its own import dialog. `onImported` refreshes the caller.
 */
export function ImportDemosButton({ onImported }: { onImported?: () => void }): ReactNode {
  if (currentEdition() === 'web') {
    return (
      <Button asChild variant="outline" size="sm">
        <Link href="/importar">
          <Upload aria-hidden />
          Importar demos
        </Link>
      </Button>
    );
  }
  return <DemoImport onImported={onImported ?? noop} />;
}
