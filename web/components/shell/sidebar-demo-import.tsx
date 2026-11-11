'use client';

import { useRouter } from 'next/navigation';
import type { ReactElement } from 'react';
import { Upload } from 'lucide-react';
import { DemoImport } from '@/components/matches/demo-import';
import { Button } from '@/components/ui/button';

/** The sidebar's Demo button: opens the same Importar demos dialog as Partidas, from any screen. */
export function SidebarDemoImport(): ReactElement {
  const router = useRouter();
  return (
    <DemoImport
      onImported={() => router.refresh()}
      trigger={(openDialog) => (
        <Button
          type="button"
          variant="outline"
          size="sm"
          title="Importar demos"
          className="min-w-0 flex-1 group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:flex-none group-data-[collapsible=icon]:px-0"
          onClick={openDialog}
        >
          <Upload aria-hidden className="stroke-[1.5]" />
          <span className="group-data-[collapsible=icon]:sr-only">Demo</span>
        </Button>
      )}
    />
  );
}
