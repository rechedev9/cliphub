import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door: old links to Nube land on Mis vídeos. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/nube']);
}
