import type { Metadata } from 'next';
import type { ReactNode } from 'react';

export const metadata: Metadata = { title: 'Colecciones' };

export default function CollectionsLayout({ children }: { children: ReactNode }): ReactNode {
  return children;
}
