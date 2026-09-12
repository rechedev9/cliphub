'use client';

import { useEffect } from 'react';
import { recordRendererError } from '@/lib/desktop-telemetry';

/** Error boundaries do not catch event handlers or rejected async operations. */
export function DesktopErrors(): null {
  useEffect(() => {
    const onError = (event: ErrorEvent): void => {
      recordRendererError('global.error', event.error instanceof Error
        ? event.error : new Error(`Uncaught renderer error: ${event.message || 'details unavailable'}`));
    };
    const onRejection = (event: PromiseRejectionEvent): void => {
      recordRendererError('global.error', event.reason instanceof Error
        ? event.reason : new Error(`Unhandled promise rejection: ${typeof event.reason === 'string' ? event.reason : 'non-error value'}`));
    };
    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onRejection);
    return () => {
      window.removeEventListener('error', onError);
      window.removeEventListener('unhandledrejection', onRejection);
    };
  }, []);
  return null;
}
