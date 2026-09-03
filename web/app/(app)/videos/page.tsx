import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door; the section moved into 01 Clips y vídeos. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/videos']);
}
