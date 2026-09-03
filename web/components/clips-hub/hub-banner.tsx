import type { ReactNode } from 'react';
import { PlugZap, RefreshCw } from 'lucide-react';
import { cn } from '@/lib/utils';
import { IconTile } from '@/components/studio/icon-tile';
import { Button } from '@/components/ui/button';

const OFFLINE_BANNER_TITLE = 'Servicio local offline';
const RETRY_NOW_LABEL = 'Reintentar ahora';

export type HubBannerProps = {
  offline: boolean;
  onRetry: () => void;
};

/** Error is a banner, never a page: the list below stays visible and inert. */
export function HubBanner({ offline, onRetry }: HubBannerProps): ReactNode {
  return (
    <div
      role="alert"
      className={cn(
        'studio-panel studio-enter flex flex-wrap items-center gap-3 px-4 py-3',
        offline ? 'border-destructive/45' : 'border-warning/45',
      )}
    >
      <IconTile icon={PlugZap} size="sm" tone={offline ? 'danger' : 'warning'} depth="inset" />
      <p className="min-w-0 flex-1 text-body-sm text-fg-2">
        <span className={cn('font-display font-bold uppercase', offline ? 'text-destructive' : 'text-warning')}>
          {offline ? OFFLINE_BANNER_TITLE : 'La última actualización falló'}
        </span>
        {' · '}
        {offline
          ? 'Arranca el servicio de ClipHub; la lista sigue tal cual estaba.'
          : 'Lo que ves puede estar desactualizado.'}
      </p>
      <Button type="button" variant="outline" size="sm" onClick={onRetry}>
        <RefreshCw aria-hidden />
        {RETRY_NOW_LABEL}
      </Button>
    </div>
  );
}
