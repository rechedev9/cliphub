import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Retired door: Ranking left Studio; the per-player numbers live in each player's sheet. */
export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/ranking']);
}
