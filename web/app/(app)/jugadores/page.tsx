import type { ReactNode } from 'react';
import Link from 'next/link';
import { UserSearch } from 'lucide-react';
import { FACEIT_PLAYERS_HREF } from '@/lib/clips/routes';
import { PlayerLibrary } from '@/components/players/player-library';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';

/** Jugadores: the players of the library's demos; the FACEIT follow list lives one level down. */
export default function PlayersPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Jugadores"
        description="La ficha de cada jugador de tus demos: sus nicks, sus equipos por fecha, sus partidas y el rol que se lee de sus rondas."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={FACEIT_PLAYERS_HREF}>
              <UserSearch aria-hidden />
              Seguimiento FACEIT
            </Link>
          </Button>
        }
      />
      <PlayerLibrary />
    </div>
  );
}
