'use client';

import type { ReactNode } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ShortcutTable } from './shortcut-table';

/** The replay's keyboard shortcuts, opened with "?" or the header button. */
export function ShortcutHelp({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-4 sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Atajos de teclado</DialogTitle>
          <DialogDescription>Funcionan mientras el Replay está en pantalla y no escribes en un campo.</DialogDescription>
        </DialogHeader>
        <ShortcutTable />
      </DialogContent>
    </Dialog>
  );
}
