import { redirect } from 'next/navigation';
import { CLIPS_HREF } from '@/lib/clips/routes';

export default function RootPage(): never {
  redirect(CLIPS_HREF);
}
