import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door: old links to Nube land on Resultados. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/nube']);
}
