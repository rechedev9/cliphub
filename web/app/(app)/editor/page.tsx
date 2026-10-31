import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door; the section was folded into the creation flows and Mis vídeos. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/editor']);
}
