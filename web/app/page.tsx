import { redirect } from 'next/navigation';
import { currentEdition, homePathFor } from '@/lib/edition';

/** The root opens the edition's home: Mis vídeos on desktop, Partidas on web. */
export default function RootPage(): never {
  redirect(homePathFor(currentEdition()));
}
