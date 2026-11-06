'use client';

import { useCallback, useMemo } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import {
  parseAnalysisScope,
  resetScopeFields,
  SCOPE_FIELDS,
  writeAnalysisScope,
  type AnalysisScope,
  type ScopeField,
} from '@/lib/analysis/scope';

/** Whose analysis the page shows. It comes from the route, never from the query. */
export type AnalysisEntity =
  | { readonly kind: 'team'; readonly id: string }
  | { readonly kind: 'player'; readonly id: string }
  | { readonly kind: 'report'; readonly id: string };

export type AnalysisScopeControl = {
  readonly entity: AnalysisEntity;
  readonly scope: AnalysisScope;
  /** Merges the patch into the scope and replaces the URL, without a new history entry or a scroll. */
  setScope: (patch: Partial<AnalysisScope>) => void;
  /** Puts the named fields back at their defaults; every field when none are named. */
  resetScope: (fields?: readonly ScopeField[]) => void;
};

/**
 * The analysis scope of the page, kept in the URL search params so every tab
 * of a team, player or report page reads the same filter and a reload or a
 * shared link restores it.
 */
export function useAnalysisScope(entity: AnalysisEntity): AnalysisScopeControl {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const query = params.toString();
  const scope = useMemo(() => parseAnalysisScope(new URLSearchParams(query)), [query]);

  const replace = useCallback(
    (next: AnalysisScope) => {
      const search = writeAnalysisScope(next, query);
      if (search === query) return;
      router.replace(search === '' ? pathname : `${pathname}?${search}`, { scroll: false });
    },
    [router, pathname, query],
  );

  const setScope = useCallback((patch: Partial<AnalysisScope>) => replace({ ...scope, ...patch }), [replace, scope]);
  const resetScope = useCallback(
    (fields: readonly ScopeField[] = SCOPE_FIELDS) => replace(resetScopeFields(scope, fields)),
    [replace, scope],
  );

  return { entity, scope, setScope, resetScope };
}
