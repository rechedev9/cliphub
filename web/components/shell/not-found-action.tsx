'use client';

import type { ReactElement } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { useDocumentTitle } from '@/components/shell/route-title';
import { editionSectionForPath, homeLabelFor, homePathFor } from '@/lib/edition';
import { currentEdition } from '@/lib/edition-routes';

export function NotFoundAction(): ReactElement {
  const edition = currentEdition();
  const section = editionSectionForPath(usePathname(), edition);
  const fallback = homePathFor(edition);
  useDocumentTitle('Página no encontrada · ClipHub');
  return (
    <Button asChild>
      <Link href={section?.href ?? fallback}>Ir a {section?.label ?? homeLabelFor(edition)}</Link>
    </Button>
  );
}
