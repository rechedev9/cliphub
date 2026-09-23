import type { CSSProperties, ReactNode } from 'react';
import { cn } from '@/lib/utils';

const MAX_LEVEL = 10;

function levelColor(level: number): string {
  if (level >= 10) return 'var(--faceit-level-10)';
  if (level >= 8) return 'var(--faceit-level-8)';
  if (level >= 4) return 'var(--faceit-level-4)';
  if (level >= 2) return 'var(--faceit-level-2)';
  return 'var(--faceit-level-1)';
}

/**
 * FACEIT-style level gauge: the ring fills level/10 in the tier colour and the
 * number stays in text ink. A solid red disc read as an error next to the red
 * "Derrota" tags, and every followed pro is level 10.
 */
export function LevelBadge({ level, className }: { level?: number; className?: string }): ReactNode {
  const known = level !== undefined && Number.isFinite(level);
  const fraction = known ? Math.min(Math.max(level, 0), MAX_LEVEL) / MAX_LEVEL : 0;
  const label = `Nivel FACEIT ${level ?? 'desconocido'}`;

  return (
    <span role="img" title={label} aria-label={label}
      style={known ? { '--level-color': levelColor(level) } as CSSProperties : undefined}
      className={cn('relative inline-grid size-7 shrink-0 place-items-center rounded-full bg-surface-1 text-meta font-bold tracking-normal tabular-nums text-fg-1', className)}>
      <svg aria-hidden viewBox="0 0 36 36" className="absolute inset-0 size-full -rotate-90">
        <circle cx="18" cy="18" r="15.5" fill="none" stroke="var(--border-subtle)" strokeWidth="3" />
        {known && fraction > 0 ? <circle cx="18" cy="18" r="15.5" fill="none" stroke="var(--level-color)" strokeWidth="3"
          strokeLinecap={fraction < 1 ? 'round' : 'butt'} pathLength={100} strokeDasharray={`${fraction * 100} 100`} /> : null}
      </svg>
      <span className="relative">{level ?? '—'}</span>
    </span>
  );
}
