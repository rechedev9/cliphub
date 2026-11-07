'use client';

import type { ReactElement } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { RESULTS_HREF } from '@/lib/clips/routes';
import { sectionForPath } from '@/lib/nav';

/** The way out of a 404: back to the section the address belongs to, else Mis vídeos. */
export function NotFoundAction(): ReactElement {
  const section = sectionForPath(usePathname());
  return (
    <Button asChild>
      <Link href={section?.href ?? RESULTS_HREF}>Ir a {section?.label ?? 'Mis vídeos'}</Link>
    </Button>
  );
}
