'use client';

import { useState, type ReactNode } from 'react';
import { Copy } from 'lucide-react';
import type { ProLineup } from '@/lib/api/pro-lineups';
import { setposCommand } from '@/lib/libreta/lineups';
import { Button } from '@/components/ui/button';

/** The console line that stands you on the lineup, with a copy button. */
export function SetposCopy({ lineup }: { lineup: Pick<ProLineup, 'origin' | 'pitch' | 'yaw'> }): ReactNode {
  const [copied, setCopied] = useState(false);
  const command = setposCommand(lineup);
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-meta text-fg-2">Posición y mira para la consola</span>
      <div className="flex items-start gap-2">
        <code className="min-w-0 flex-1 rounded-sm bg-surface-3 px-2 py-1 font-mono text-meta break-all text-fg-1">{command}</code>
        <Button
          type="button"
          variant="outline"
          size="xs"
          onClick={() => {
            void navigator.clipboard
              .writeText(command)
              .then(() => setCopied(true))
              .catch(() => setCopied(false));
          }}
        >
          <Copy aria-hidden />
          {copied ? 'Copiado' : 'Copiar'}
        </Button>
      </div>
    </div>
  );
}
