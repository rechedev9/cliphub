import type { ReactNode } from 'react';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const TRACK_ON = {
  primary: { track: 'border-primary bg-primary/25', thumb: 'bg-primary' },
  stream: { track: 'border-stream bg-stream/25', thumb: 'bg-stream' },
} as const;

export type SwitchProps = {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  /** The track colour when on: the app accent, or the stream tone inside the stream editor. */
  tone?: keyof typeof TRACK_ON;
  'aria-label'?: string;
  'aria-labelledby'?: string;
};

/** The one on/off control in Studio: a square track whose thumb slides right when on. */
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
        'relative inline-flex h-5 w-9 shrink-0 items-center rounded-md border transition-colors duration-(--dur-fast) ease-standard disabled:pointer-events-none disabled:opacity-50',
        checked ? TRACK_ON[tone].track : 'border-border-strong bg-surface-3',
        FOCUS_RING,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'block size-3 rounded-[1px] transition-transform duration-(--dur-fast) ease-standard',
          checked ? ['translate-x-[1.1875rem]', TRACK_ON[tone].thumb] : ['translate-x-[0.1875rem] bg-fg-3'],
        )}
      />
    </button>
  );
}
