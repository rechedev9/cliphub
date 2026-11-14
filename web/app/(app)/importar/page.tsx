import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { WebImport } from '@/components/clipmind/web-import';

export const metadata: Metadata = { title: 'Importar demos' };

export default function Page(): ReactNode {
  return (
    <div className="flex flex-col gap-7">
      <StudioPageHeader title="Importar demos" description="Tu navegador analiza las demos y guarda el resultado en el espacio de tu equipo." />
      <WebImport />
    </div>
  );
}
