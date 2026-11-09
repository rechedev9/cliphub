import type { ReactNode } from 'react';
import Link from 'next/link';
import { UserSearch } from 'lucide-react';
import { FACEIT_PLAYERS_HREF } from '@/lib/clips/routes';
import { StudioPageHeader } from '@/components/studio/page-header';
import { TeamLibrary } from '@/components/teams/team-library';
import { Button } from '@/components/ui/button';

/** Equipos: team records built from the clan names of the library's demos. */
export default function TeamsPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Equipos"
        description="La ficha de cada equipo con su plantilla, sus partidas y su rendimiento por mapa, el punto de partida para preparar un rival."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={FACEIT_PLAYERS_HREF}>
              <UserSearch aria-hidden />
              Seguimiento FACEIT
            </Link>
          </Button>
        }
      />
      <TeamLibrary />
    </div>
  );
}
