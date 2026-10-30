import { redirect } from 'next/navigation';
import { RESULTS_HREF } from '@/lib/clips/routes';

/** Local Studio has no marketing root: open Mis vídeos, which offers the creation flows when empty. */
export default function RootPage(): never {
  redirect(RESULTS_HREF);
}
