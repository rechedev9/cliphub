'use client';

import { useState, type ReactNode } from 'react';
import { FileDown, Trash2, X } from 'lucide-react';
import { LIBRETA_STATUSES, type LibretaStatus } from '@/lib/api/libreta';
import { STATUS_LABEL } from '@/lib/libreta/status';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';

/** What can be done to the rows chosen in a Libreta list. `extra` holds list-specific actions. */
export function BulkBar({
  count,
  noun,
  busy,
  onStatus,
  onExport,
  onDelete,
  onClear,
  extra,
}: {
  count: number;
  noun: string;
  busy: boolean;
  onStatus: (status: LibretaStatus) => void;
  onExport?: () => void;
  onDelete: () => void;
  onClear: () => void;
  extra?: ReactNode;
}): ReactNode {
  const [confirming, setConfirming] = useState(false);
  return (
    <div role="region" aria-label="Acciones de la selección" className="studio-panel flex flex-wrap items-center gap-2 px-3 py-2">
      <span className="text-body-sm font-medium text-fg-1">
        {count} {noun} en la selección
      </span>
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
      <Button type="button" variant="ghost" size="xs" disabled={busy} onClick={() => setConfirming(true)}>
        <Trash2 aria-hidden />
        Eliminar
      </Button>
      <Button type="button" variant="ghost" size="xs" className="ml-auto" disabled={busy} onClick={onClear}>
        <X aria-hidden />
        Quitar la selección
      </Button>
      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              ¿Eliminar {count} {noun}?
            </DialogTitle>
            <DialogDescription>No se puede deshacer. Para quitar algo de la lista sin borrarlo, cambia su estado a Archivo.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirming(false)}>
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                setConfirming(false);
                onDelete();
              }}
            >
              Eliminar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
