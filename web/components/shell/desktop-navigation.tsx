'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { desktopNavigation } from '@/lib/desktop-navigation';

/** Opens the pages an assistant asks for with the app's router, without the full reload that drops the app's state. */
export function DesktopNavigation(): null {
  const router = useRouter();
  useEffect(() => desktopNavigation()?.((path) => router.push(path)), [router]);
  return null;
}
