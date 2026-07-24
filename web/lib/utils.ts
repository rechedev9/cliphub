import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

/**
 * The project's `--text-*` type scale (app/globals.css `@theme`).
 *
 * tailwind-merge only knows Tailwind's built-in font sizes, so it files every
 * custom `text-<step>` under the *text-colour* group. Without this list,
 * `cn('text-meta', 'text-fg-2')` silently drops the size and
 * `cn('text-fg-1', 'text-label')` silently drops the colour. Keep in sync with
 * the `--text-*` keys in globals.css.
 */
const TYPE_SCALE_STEPS = [
  'meta',
  'label',
  'body-sm',
  'body',
  'body-lg',
  'title',
  'section',
  'display-sm',
  'display',
  'hero',
  'stat',
] as const;

const twMerge = extendTailwindMerge({
  extend: { classGroups: { 'font-size': [{ text: [...TYPE_SCALE_STEPS] }] } },
});

/** Merge class names, resolving Tailwind conflicts (last wins). */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
