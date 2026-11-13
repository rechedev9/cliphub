'use client';

import type { ReactElement } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { currentEdition, editionSectionForPath, homeLabelFor, homePathFor } from '@/lib/edition';

/** The way out of a 404: back to the section the address belongs to, else the edition's home. */
export function NotFoundAction(): ReactElement {
  const edition = currentEdition();
  const section = editionSectionForPath(usePathname(), edition);
  const fallback = homePathFor(edition);
  return (
    <Button asChild>
      <Link href={section?.href ?? fallback}>Ir a {section?.label ?? homeLabelFor(edition)}</Link>
    </Button>
  );
}
