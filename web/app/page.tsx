import { redirect } from 'next/navigation';

/** Local Studio has no marketing root: open Inicio. */
export default function RootPage(): never {
  redirect('/onboarding');
}
