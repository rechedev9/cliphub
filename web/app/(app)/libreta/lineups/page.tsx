import { redirect } from 'next/navigation';
import { LIBRETA_TABS, libretaTabHref } from '@/lib/clips/routes';

/** The lineup list lives in the Libreta's Lineups tab. */
export default function LineupsPage(): never {
  redirect(libretaTabHref(LIBRETA_TABS.lineups));
}
