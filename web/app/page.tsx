import { redirect } from 'next/navigation';
import { homePathFor } from '@/lib/edition';
import { currentEdition } from '@/lib/edition-routes';

/** The root opens the edition's home: Mis vídeos on desktop, Partidas on web. */
export default function RootPage(): never {
  redirect(homePathFor(currentEdition()));
}
