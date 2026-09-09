import type { ReactNode } from 'react';
import type { Preset } from '@/lib/api/types';
import { cn } from '@/lib/utils';

/** An explicitly illustrative comparison of the registry's HUD and colour choices. */
export function PresetPreview({ preset, className }: { preset: Preset; className?: string }): ReactNode {
  const aggressive = preset.name === 'viral-aggressive-60';
  const hud = preset.hudMode ?? ({
    'viral-60-clean': 'deathnotices', 'viral-aggressive-60': 'deathnotices',
    'clean-pov-60': 'clean', 'full-hud-60': 'gameplay',
  } as Record<string, string>)[preset.name];
  return (
    <div aria-hidden className={cn('relative aspect-[9/16] shrink-0 overflow-hidden rounded border border-border-strong bg-surface-0', className)}>
      <svg viewBox="0 0 90 160" className="size-full" preserveAspectRatio="xMidYMid slice">
        <rect width="90" height="160" fill={aggressive ? '#4b2768' : '#33465b'} />
        <path d="M0 78L43 61L90 78V160H0Z" fill={aggressive ? '#6e497d' : '#657386'} />
        <path d="M0 23L30 45V113L0 146Z" fill={aggressive ? '#c174a1' : '#98a5ad'} />
        <path d="M90 33L66 48V112L90 143Z" fill={aggressive ? '#8f5289' : '#768591'} />
        <path d="M35 73H58V105H35Z" fill="#182433" />
        <path d="M46 160L55 117L67 109L77 127L79 160Z" fill="#142331" />
        <path d="M41 82H49M45 78V86" stroke="#a5ee80" strokeWidth="1" />
        {hud === 'gameplay' ? <>
          <rect x="4" y="5" width="21" height="21" rx="2" fill="#101c2c" />
          <path d="M9 21V10H19V17H14V24" fill="none" stroke="#aac4d8" strokeWidth="2" />
          <rect x="5" y="149" width="22" height="4" fill="#cfe8f2" />
          <rect x="66" y="149" width="19" height="4" fill="#cfe8f2" />
        </> : null}
        {hud === 'gameplay' || hud === 'deathnotices' ? <>
          <rect x="48" y="9" width="38" height="5" fill="#16222d" />
          <rect x="53" y="11" width="12" height="1" fill="#22d9ee" />
          <rect x="72" y="11" width="10" height="1" fill="#f4b873" />
        </> : null}
        {aggressive ? <path d="M0 88H90M0 91H90" stroke="#ff2d78" opacity=".35" /> : null}
      </svg>
    </div>
  );
}
