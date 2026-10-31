import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Inicio retired: Mis vídeos offers the creation flows when it is empty. */
export default function OnboardingPage(): never {
  redirect(RETIRED_ROUTES['/onboarding']);
}
