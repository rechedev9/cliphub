import { redirect } from 'next/navigation';
import { RETIRED_ROUTES } from '@/lib/nav';

/** Inicio retired: the hub at 01 is the single door. */
export default function OnboardingPage(): never {
  redirect(RETIRED_ROUTES['/onboarding']);
}
