'use client';

import { useEffect, useState, useSyncExternalStore, type ReactElement } from 'react';
import { ArrowDownToLine, RefreshCw, TriangleAlert } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  APP_UPDATE_STATE,
  appUpdateAction,
  appUpdateLabel,
  appUpdateTitle,
  appUpdateVisible,
  parseAppUpdateStatus,
  type AppUpdateStatus,
} from '@/lib/app-update';
import { getDesktopUpdateBridge } from '@/lib/desktop-update';
import {
  serverShellActivitySnapshot,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';

const IDLE: AppUpdateStatus = { state: APP_UPDATE_STATE.idle };

export function AppUpdateControl(): ReactElement | null {
  const [status, setStatus] = useState<AppUpdateStatus>(IDLE);
  const activity = useSyncExternalStore(
    subscribeToShellActivity,
    shellActivitySnapshot,
    serverShellActivitySnapshot,
  );

  useEffect(() => {
    const bridge = getDesktopUpdateBridge();
    if (bridge === null) return undefined;
    let cancelled = false;
    void bridge.getStatus().then((value) => {
      const next = parseAppUpdateStatus(value);
      if (!cancelled && next !== null) setStatus(next);
    }).catch(() => undefined);
    const stop = bridge.onStatus((value) => {
      const next = parseAppUpdateStatus(value);
      if (next !== null) setStatus(next);
    });
    return () => {
      cancelled = true;
      stop();
    };
  }, []);

  if (!appUpdateVisible(status)) return null;

  const jobsBusy = activity.jobs.length > 0;
  const label = appUpdateLabel(status);
  if (label === null) return null;

  const action = appUpdateAction(status);
  const busy = status.state === APP_UPDATE_STATE.downloading
    || status.state === APP_UPDATE_STATE.installing
    || status.state === APP_UPDATE_STATE.checking;
  const blocked = status.state === APP_UPDATE_STATE.ready && jobsBusy;
  const title = appUpdateTitle(status, jobsBusy);

  return (
    <Button
      type="button"
      variant={buttonVariant(status.state)}
      size="sm"
      className="h-10 shrink-0"
      loading={busy}
      loadingText={label}
      disabled={blocked}
      title={title}
      aria-label={title}
      aria-live="polite"
      data-testid="app-update-control"
      onClick={() => {
        const bridge = getDesktopUpdateBridge();
        if (bridge === null || action === null || blocked) return;
        if (action === 'check') {
          void bridge.check();
          return;
        }
        void bridge.install();
      }}
    >
      <StatusIcon state={status.state} />
      <span className="font-[family-name:var(--font-display)] text-meta font-semibold tracking-wider uppercase">
        {label}
      </span>
    </Button>
  );
}

function buttonVariant(
  state: AppUpdateStatus['state'],
): 'outline-primary' | 'success' | 'warning' | 'ghost' {
  if (state === APP_UPDATE_STATE.ready || state === APP_UPDATE_STATE.installing) return 'success';
  if (state === APP_UPDATE_STATE.error) return 'warning';
  if (state === APP_UPDATE_STATE.checking) return 'ghost';
  return 'outline-primary';
}

function StatusIcon({ state }: { state: AppUpdateStatus['state'] }): ReactElement {
  if (state === APP_UPDATE_STATE.error) return <TriangleAlert aria-hidden />;
  if (state === APP_UPDATE_STATE.ready) return <RefreshCw aria-hidden />;
  return <ArrowDownToLine aria-hidden />;
}
