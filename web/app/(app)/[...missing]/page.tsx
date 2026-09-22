import { notFound } from 'next/navigation';

/**
 * Unmatched URLs otherwise fall through to the root `not-found.tsx`, which
 * renders outside the shell. Catching them here keeps the sidebar and shows
 * the in-shell 404 from `(app)/not-found.tsx`.
 */
export default function MissingRoute(): never {
  notFound();
}
