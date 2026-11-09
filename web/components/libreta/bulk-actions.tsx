'use client';

import type { ReactNode } from 'react';
import { FileDown } from 'lucide-react';
import { LIBRETA_STATUSES, type LibretaStatus } from '@/lib/api/libreta';
import { STATUS_LABEL } from '@/lib/libreta/status';
import type { CountNoun } from '@/lib/records/selection';
import { DeleteChosenButton } from '@/components/records/delete-chosen-button';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** What can be done to the rows chosen in a Libreta list, for its table's selection bar. `extra` holds list-specific actions. */
export function LibretaBulkActions({
  count,
  noun,
  busy,
  onStatus,
  onExport,
  onDelete,
  extra,
}: {
  count: number;
  noun: CountNoun;
  busy: boolean;
  onStatus: (status: LibretaStatus) => void;
  onExport?: () => void;
  onDelete: () => void;
  extra?: ReactNode;
}): ReactNode {
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="outline" size="xs" disabled={busy}>
            Cambiar estado
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {LIBRETA_STATUSES.map((status) => (
            <DropdownMenuItem key={status} onSelect={() => onStatus(status)}>
              {STATUS_LABEL[status]}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {extra}
      {onExport ? (
        <Button type="button" variant="outline" size="xs" disabled={busy} onClick={onExport}>
          <FileDown aria-hidden />
          Exportar
        </Button>
      ) : null}
      <DeleteChosenButton
        count={count}
        noun={noun}
        verb="Eliminar"
        disabled={busy}
        description="No se puede deshacer. Para quitar algo de la lista sin borrarlo, cambia su estado a Archivo."
        onConfirm={onDelete}
      />
    </>
  );
}
