'use client';

import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Search } from 'lucide-react';
import { searchLibrary } from '@/lib/api/search';
import { resultRow, SEARCH_GROUP_LABEL, sectionRows, type SearchRow } from '@/lib/search/results';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

const SEARCH_DELAY_MS = 180;

type Found = { query: string; rows: SearchRow[]; failed: boolean };

/** Ctrl+K anywhere in the app: sections of the rail and the library's records. */
export function GlobalSearch(): ReactNode {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent): void => {
      if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        aria-label="Buscar en Studio"
        title="Buscar · Ctrl K"
        className="text-fg-2 hover:text-fg-1 max-sm:w-10 max-sm:px-0"
        onClick={() => setOpen(true)}
      >
        <Search aria-hidden />
        <span className="max-sm:sr-only">Buscar</span>
        <kbd className="ml-1 rounded border border-border-subtle px-1 text-meta text-fg-3 max-md:hidden">Ctrl K</kbd>
      </Button>
      {open ? <SearchDialog onClose={() => setOpen(false)} /> : null}
    </>
  );
}

function emptyText({ trimmed, pending, failed }: { trimmed: string; pending: boolean; failed: boolean }): string {
  if (trimmed === '') return 'Un nombre de mapa o de granada acota los resultados.';
  if (pending) return 'Buscando…';
  return failed ? 'No se pudo buscar. Prueba otra vez.' : 'Nada coincide con esa búsqueda.';
}

function SearchDialog({ onClose }: { onClose: () => void }): ReactNode {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const [found, setFound] = useState<Found>({ query: '', rows: [], failed: false });
  const [active, setActive] = useState(0);
  const listRef = useRef<HTMLUListElement>(null);

  useEffect(() => {
    const q = query.trim();
    if (q === '') return;
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      searchLibrary(q, controller.signal)
        .then((results) => setFound({ query: q, rows: results.map(resultRow), failed: false }))
        .catch(() => {
          if (!controller.signal.aborted) setFound({ query: q, rows: [], failed: true });
        });
    }, SEARCH_DELAY_MS);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [query]);

  const trimmed = query.trim();
  const rows = useMemo(
    () => (trimmed === '' ? [] : [...sectionRows(trimmed), ...(found.query === trimmed ? found.rows : [])]),
    [trimmed, found],
  );
  const current = Math.min(active, Math.max(rows.length - 1, 0));

  const go = (row: SearchRow | undefined): void => {
    if (row === undefined) return;
    onClose();
    router.push(row.href);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>): void => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const next = rows.length === 0 ? 0 : (current + (event.key === 'ArrowDown' ? 1 : rows.length - 1)) % rows.length;
      setActive(next);
      listRef.current?.querySelector(`[data-index="${next}"]`)?.scrollIntoView({ block: 'nearest' });
    } else if (event.key === 'Enter') {
      event.preventDefault();
      go(rows[current]);
    }
  };

  const pending = trimmed !== '' && found.query !== trimmed;
  return (
    <Dialog open onOpenChange={(next) => (next ? null : onClose())}>
      <DialogContent className="top-[15%] translate-y-0 gap-3 p-3 sm:max-w-xl" showCloseButton={false}>
        <DialogHeader className="sr-only">
          <DialogTitle>Buscar en Studio</DialogTitle>
          <DialogDescription>Jugadas, lineups, llamadas, equipos, jugadores, partidas y secciones.</DialogDescription>
        </DialogHeader>
        <Input
          autoFocus
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive(0);
          }}
          onKeyDown={onKeyDown}
          placeholder="Busca una jugada, un lineup, un equipo… prueba «humo ventana mirage»"
          aria-label="Buscar"
        />
        {rows.length === 0 ? (
          <p className="px-1 py-2 text-meta text-fg-3">{emptyText({ trimmed, pending, failed: found.failed })}</p>
        ) : (
          <ul ref={listRef} className="flex max-h-[60vh] flex-col overflow-y-auto" role="listbox" aria-label="Resultados">
            {rows.map((row, index) => (
              <li key={row.key} role="presentation">
                {index === 0 || rows[index - 1]?.group !== row.group ? (
                  <p className="px-2 pt-2 pb-1 text-meta font-medium text-fg-3">{SEARCH_GROUP_LABEL[row.group]}</p>
                ) : null}
                <button
                  type="button"
                  role="option"
                  aria-selected={index === current}
                  data-index={index}
                  onMouseEnter={() => setActive(index)}
                  onClick={() => go(row)}
                  className={cn(
                    'flex w-full flex-col items-start rounded-md px-2 py-1.5 text-left',
                    index === current ? 'bg-surface-3 text-fg-1' : 'text-fg-2',
                  )}
                >
                  <span className="truncate text-body-sm font-medium">{row.title}</span>
                  {row.subtitle === '' ? null : <span className="truncate text-meta text-fg-3">{row.subtitle}</span>}
                </button>
              </li>
            ))}
          </ul>
        )}
      </DialogContent>
    </Dialog>
  );
}
