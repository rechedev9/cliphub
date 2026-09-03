import { redirect } from 'next/navigation';
import { CLIPS_HREF } from '@/lib/clips/routes';

/** Local Studio has no marketing root: open 01 Clips y vídeos. */
export default function RootPage(): never {
  redirect(CLIPS_HREF);
}
