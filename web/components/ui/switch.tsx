import type { ReactNode } from 'react';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const TRACK_ON = { primary: 'bg-primary', stream: 'bg-stream' } as const;

export type SwitchProps = {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  /** The track colour when on: the app accent, or the stream tone inside the stream editor. */
  tone?: keyof typeof TRACK_ON;
  'aria-label'?: string;
  'aria-labelledby'?: string;
};

/** The one on/off control in Studio: a round track whose thumb slides right when on. */
export function Switch({ checked, onCheckedChange, disabled = false, tone = 'primary', ...aria }: SwitchProps): ReactNode {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      {...aria}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors duration-(--dur-fast) ease-standard disabled:pointer-events-none disabled:opacity-50',
        checked ? TRACK_ON[tone] : 'bg-border-strong',
        FOCUS_RING,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'block size-5 rounded-full bg-fg-1 shadow-sm transition-transform duration-(--dur-fast) ease-standard',
          checked ? 'translate-x-5.5' : 'translate-x-0.5',
        )}
      />
    </button>
  );
}
