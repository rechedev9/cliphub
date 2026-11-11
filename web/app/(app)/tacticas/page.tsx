import { redirect } from 'next/navigation';

/** The Editor de tácticas has one screen: /tacticas goes to the builder. */
export default function TacticsPage(): never {
  redirect('/tacticas/nueva');
}
