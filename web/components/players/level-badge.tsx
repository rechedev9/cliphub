import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export function LevelBadge({ level, className }: { level?: number; className?: string }): ReactNode {
  let tone = 'border-border-strong bg-surface-3 text-fg-2';
  if (level !== undefined && level >= 7) tone = 'border-destructive bg-destructive text-destructive-foreground';
  else if (level !== undefined && level >= 4) tone = 'border-warning bg-warning text-warning-foreground';
  else if (level !== undefined && level >= 2) tone = 'border-success bg-success text-success-foreground';

  return (
    <span title={`Nivel FACEIT ${level ?? 'desconocido'}`} aria-label={`Nivel FACEIT ${level ?? 'desconocido'}`}
      className={cn('inline-flex size-6 shrink-0 items-center justify-center rounded-full border text-meta font-bold tracking-normal tabular-nums', tone, className)}>
      {level ?? '—'}
    </span>
  );
}
