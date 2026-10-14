import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

export default function RetiredPage(): never {
  redirect(RETIRED_ROUTES['/upload']);
}
