'use client';

import { useCallback } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { tabFromQuery, writeTab } from '@/lib/analysis/context-bar';

/**
 * The open tab of a record page, kept in `?tab=` so switching team or player
 * from the context bar, a reload or a shared link lands on the same tab.
 */
export function useRecordTab(tabs: readonly string[], fallback: string): [string, (tab: string) => void] {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const query = params.toString();
  const tab = tabFromQuery(params, tabs, fallback);
  const setTab = useCallback(
    (next: string) => {
      const search = writeTab(query, next, fallback);
      router.replace(search === '' ? pathname : `${pathname}?${search}`, { scroll: false });
    },
    [router, pathname, query, fallback],
  );
  return [tab, setTab];
}
