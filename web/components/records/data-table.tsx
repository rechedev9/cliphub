'use client';

import { useState, type FormEvent, type MouseEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowDown, ArrowUp, ArrowUpDown, BookmarkPlus, Check, Columns3, Eye, Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import type { TableViewControl } from '@/components/records/use-table-view';
import { nextSort, sortRows, type SortValue } from '@/lib/records/table-view';
import { cn } from '@/lib/utils';

export type DataTableColumn<Row> = {
  readonly id: string;
  readonly header: string;
  readonly cell: (row: Row) => ReactNode;
  /** Present when the column can be sorted. */
  readonly sortValue?: (row: Row) => SortValue;
  readonly align?: 'start' | 'end';
  /** A required column cannot be hidden from the column menu. */
  readonly required?: boolean;
  readonly className?: string;
};

export type DataTableProps<Row> = {
  /** Accessible name of the table, such as "Partidas". */
  label: string;
  rows: readonly Row[];
  columns: readonly DataTableColumn<Row>[];
  rowKey: (row: Row) => string;
  view: TableViewControl;
  /** The record a row opens, if any; the first visible cell becomes its link. */
  rowHref?: (row: Row) => string | undefined;
  /** Rendered instead of the body when no row is left. */
  empty?: ReactNode;
};

/** Dense, sortable record list with configurable columns and saved views. */
export function DataTable<Row>({ label, rows, columns, rowKey, view, rowHref, empty }: DataTableProps<Row>): ReactNode {
  const router = useRouter();
  const { sort, hidden } = view.state;
  const visible = columns.filter((column) => column.required === true || !hidden.includes(column.id));
  const sorted = sortRows(rows, sort, (row, id) => columns.find((column) => column.id === id)?.sortValue?.(row) ?? null);

  // A click anywhere on the row opens it; clicks on the row's own controls keep their meaning.
  function openRow(event: MouseEvent<HTMLTableRowElement>, href: string): void {
    if (event.target instanceof Element && event.target.closest('a, button, input, [role="menuitem"]')) return;
    router.push(href);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-end gap-2">
        <SavedViewsMenu view={view} />
        <ColumnsMenu columns={columns} hidden={hidden} onToggle={view.toggleColumn} />
      </div>
      <div className="studio-panel overflow-x-auto">
        <table aria-label={label} className="w-full border-collapse text-left text-body-sm">
          <thead className="sticky top-0 bg-surface-2">
            <tr className="border-b border-border">
              {visible.map((column) => {
                const direction = sort?.column === column.id ? sort.direction : null;
                let ariaSort: 'ascending' | 'descending' | 'none' = 'none';
                if (direction === 'asc') ariaSort = 'ascending';
                if (direction === 'desc') ariaSort = 'descending';
                return (
                  <th
                    key={column.id}
                    scope="col"
                    aria-sort={column.sortValue ? ariaSort : undefined}
                    className={cn('h-9 px-3 font-medium whitespace-nowrap text-fg-3', column.align === 'end' && 'text-right')}
                  >
                    {column.sortValue ? (
                      <button
                        type="button"
                        onClick={() => view.setSort(nextSort(sort, column.id))}
                        className={cn(
                          'inline-flex items-center gap-1 rounded-sm hover:text-fg-1',
                          direction !== null && 'text-fg-1',
                          column.align === 'end' && 'flex-row-reverse',
                        )}
                      >
                        {column.header}
                        <SortGlyph direction={direction} />
                      </button>
                    ) : (
                      column.header
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          {sorted.length === 0 ? null : (
            <tbody>
              {sorted.map((row) => {
                const href = rowHref?.(row);
                return (
                  <tr
                    key={rowKey(row)}
                    onClick={href === undefined ? undefined : (event) => openRow(event, href)}
                    className={cn(
                      'border-b border-border-subtle last:border-b-0',
                      href !== undefined && 'cursor-pointer hover:bg-surface-3',
                    )}
                  >
                    {visible.map((column, index) => (
                      <td
                        key={column.id}
                        className={cn('h-10 px-3 align-middle text-fg-1', column.align === 'end' && 'text-right', column.className)}
                      >
                        {index === 0 && href !== undefined ? (
                          <Link href={href} className="font-medium text-fg-1 hover:text-primary hover:underline">
                            {column.cell(row)}
                          </Link>
                        ) : (
                          column.cell(row)
                        )}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          )}
        </table>
        {sorted.length === 0 ? <div className="p-4">{empty}</div> : null}
      </div>
    </div>
  );
}

function SortGlyph({ direction }: { direction: 'asc' | 'desc' | null }): ReactNode {
  if (direction === 'asc') return <ArrowUp className="size-3.5" aria-hidden />;
  if (direction === 'desc') return <ArrowDown className="size-3.5" aria-hidden />;
  return <ArrowUpDown className="size-3.5 text-fg-4" aria-hidden />;
}

function ColumnsMenu<Row>({
  columns,
  hidden,
  onToggle,
}: {
  columns: readonly DataTableColumn<Row>[];
  hidden: readonly string[];
  onToggle: (column: string) => void;
}): ReactNode {
  const optional = columns.filter((column) => column.required !== true);
  if (optional.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button type="button" variant="outline" size="xs">
          <Columns3 aria-hidden />
          Columnas
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-52">
        <DropdownMenuLabel>Columnas visibles</DropdownMenuLabel>
        {optional.map((column) => (
          <DropdownMenuItem
            key={column.id}
            onSelect={(event) => {
              // Keep the menu open so several columns can be toggled in a row.
              event.preventDefault();
              onToggle(column.id);
            }}
          >
            <span className="flex-1">{column.header}</span>
            {hidden.includes(column.id) ? null : <Check aria-label="Visible" className="text-primary" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function SavedViewsMenu({ view }: { view: TableViewControl }): ReactNode {
  const [naming, setNaming] = useState(false);
  const [name, setName] = useState('');
  const active = view.views.find((entry) => entry.id === view.activeViewId);

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    if (name.trim() === '') return;
    view.saveView(name);
    setNaming(false);
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="outline" size="xs">
            <Eye aria-hidden />
            {active ? active.name : 'Vistas'}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-64">
          <DropdownMenuLabel>Vistas guardadas</DropdownMenuLabel>
          {view.views.length === 0 ? (
            <p className="px-2 py-1.5 text-body-sm text-fg-3">Guarda el orden, las columnas y los filtros actuales.</p>
          ) : (
            view.views.map((entry) => (
              <DropdownMenuItem key={entry.id} onSelect={() => view.applyView(entry.id)} className="pr-1">
                <span className="min-w-0 flex-1 truncate">{entry.name}</span>
                {entry.id === view.activeViewId ? <Check aria-label="Vista actual" className="text-primary" /> : null}
                <button
                  type="button"
                  aria-label={`Borrar la vista ${entry.name}`}
                  className="grid size-7 place-items-center rounded-sm text-fg-3 hover:bg-surface-3 hover:text-destructive"
                  // The item would otherwise apply the view on the same press that deletes it.
                  onPointerDown={(event) => event.stopPropagation()}
                  onPointerUp={(event) => event.stopPropagation()}
                  onClick={(event) => {
                    event.stopPropagation();
                    view.deleteView(entry.id);
                  }}
                >
                  <Trash2 className="size-3.5" aria-hidden />
                </button>
              </DropdownMenuItem>
            ))
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={() => {
              setName(active?.name ?? '');
              setNaming(true);
            }}
          >
            <BookmarkPlus aria-hidden />
            Guardar vista actual
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={naming} onOpenChange={setNaming}>
        <DialogContent className="sm:max-w-md">
          <form onSubmit={submit} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardar vista</DialogTitle>
              <DialogDescription>
                Guarda el orden, las columnas y los filtros de esta tabla en este PC. Con el mismo nombre se reemplaza.
              </DialogDescription>
            </DialogHeader>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Nombre
              <Input
                autoFocus
                value={name}
                maxLength={60}
                onChange={(event) => setName(event.target.value)}
                placeholder="Pistols en Mirage"
              />
            </label>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setNaming(false)}>
                Cancelar
              </Button>
              <Button type="submit" disabled={name.trim() === ''}>
                Guardar
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
