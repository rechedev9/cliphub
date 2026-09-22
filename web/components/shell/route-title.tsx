'use client';

import { createContext, useContext, useEffect, useState, type Dispatch, type ReactNode, type SetStateAction } from 'react';
import { usePathname } from 'next/navigation';

type RouteTitleValue = { pathname: string; title: string } | null;
const TitleContext = createContext<RouteTitleValue>(null);
const SetTitleContext = createContext<Dispatch<SetStateAction<RouteTitleValue>> | null>(null);

/** A page supplies its already-loaded name without another request from the shell. */
export function RouteTitleProvider({ children }: { children: ReactNode }): ReactNode {
  const [value, setValue] = useState<RouteTitleValue>(null);
  return <SetTitleContext.Provider value={setValue}><TitleContext.Provider value={value}>{children}</TitleContext.Provider></SetTitleContext.Provider>;
}

export function useRouteTitle(title: string | undefined): void {
  const pathname = usePathname();
  const setTitle = useContext(SetTitleContext);
  useEffect(() => {
    if (!setTitle || !title) return;
    const value = { pathname, title };
    setTitle(value);
    return () => setTitle((current) => current === value ? null : current);
  }, [pathname, title, setTitle]);
}

/**
 * Names the browser tab once a client page knows its subject, where the
 * segment metadata can only give a static title. The static title is handed
 * back on the way out only while the tab still shows ours: by the time an
 * unmount cleanup runs, the next route may already have written its own.
 */
export function useDocumentTitle(title: string | undefined): void {
  useEffect(() => {
    if (!title) return;
    const previous = document.title;
    document.title = title;
    return () => {
      if (document.title === title) document.title = previous;
    };
  }, [title]);
}

export function useCurrentRouteTitle(): string | null {
  const pathname = usePathname();
  const value = useContext(TitleContext);
  return value?.pathname === pathname ? value.title : null;
}
