import type { Metadata } from 'next';
import type { ReactNode } from 'react';

export const metadata: Metadata = { title: 'Editor de tácticas' };

export default function TacticsLayout({ children }: { children: ReactNode }): ReactNode {
  return children;
}
