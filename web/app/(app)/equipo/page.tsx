import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import { StudioPageHeader } from '@/components/studio/page-header';
import { TeamView } from '@/components/clipmind/team-view';

export const metadata: Metadata = { title: 'Equipo' };

export default function Page(): ReactNode {
  return (
    <div className="flex flex-col gap-7">
      <StudioPageHeader title="Equipo" description="Quién comparte este espacio de análisis y cómo invitar a más gente." />
      <TeamView />
    </div>
  );
}
