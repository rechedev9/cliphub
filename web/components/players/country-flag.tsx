import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** The bundled flag-icons flag of a country, served by the local service; nothing for a code without one. */
export function CountryFlag({ code, decorative = false, className }: {
  code: string | undefined;
  /** Set when the code is already written beside the flag. */
  decorative?: boolean;
  className?: string;
}): ReactNode {
  const country = code?.trim().toLowerCase() ?? '';
  if (!/^[a-z]{2}$/.test(country)) return null;
  const label = country.toUpperCase();
  return (
    <img src={`/api/faceit/flags/${country}`} alt={decorative ? '' : label} title={decorative ? undefined : label}
      width={20} height={15} loading="lazy" decoding="async"
      onError={(event) => { event.currentTarget.hidden = true; }}
      className={cn('h-[15px] w-5 shrink-0 rounded-[2px] object-cover ring-1 ring-border-subtle', className)} />
  );
}
