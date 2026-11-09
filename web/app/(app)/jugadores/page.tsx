import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door: the player list left the rail; each player's sheet opens from Partidas, Equipos and Informes. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/jugadores']);
}
