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

/** The titles of the mounted `useDocumentTitle` callers; only the newest keeps the tab, so two never fight over it. */
const titleClaims: { title: string }[] = [];

/**
 * Names the browser tab once a client page knows its subject, where the
 * segment metadata can only give a static title. On the way out the tab goes
 * back to the next claim, or to the static title while the tab still shows ours:
 * by the time an unmount cleanup runs, the next route may already have written its own.
 */
export function useDocumentTitle(title: string | undefined): void {
  useEffect(() => {
    if (!title) return;
    const claim = { title };
    titleClaims.push(claim);
    const previous = document.title;
    const path = window.location.pathname;
    document.title = title;
    // A query-only navigation (a tab in `?tab=`) re-applies the segment's static title; keep the subject's name.
    const observer = new MutationObserver(() => {
      if (titleClaims.at(-1) === claim && window.location.pathname === path && document.title !== title) document.title = title;
    });
    observer.observe(document.head, { subtree: true, childList: true, characterData: true });
    return () => {
      observer.disconnect();
      titleClaims.splice(titleClaims.indexOf(claim), 1);
      if (document.title === title) document.title = titleClaims.at(-1)?.title ?? previous;
    };
  }, [title]);
}

export function useCurrentRouteTitle(): string | null {
  const pathname = usePathname();
  const value = useContext(TitleContext);
  return value?.pathname === pathname ? value.title : null;
}
