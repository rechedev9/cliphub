import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door: the hub split into Short, Vídeo largo and Mis vídeos. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/clips']);
}
