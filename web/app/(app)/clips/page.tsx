import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door: the hub split into Shorts, Vídeos largos and Resultados. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/clips']);
}
