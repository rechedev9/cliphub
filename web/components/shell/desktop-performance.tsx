'use client';

import { useEffect, type ReactElement } from 'react';
import { recordRendererSpan } from '@/lib/desktop-telemetry';

/** Emits coarse initial-navigation timings; Electron main samples them at 10%. */
export function DesktopPerformance(): ReactElement | null {
  useEffect(() => {
    const report = (): void => {
      const navigation = performance.getEntriesByType('navigation')[0];
      if (!(navigation instanceof PerformanceNavigationTiming)) return;
      if (navigation.domContentLoadedEventEnd > 0) {
        recordRendererSpan('navigation.dom_content_loaded', navigation.domContentLoadedEventEnd);
      }
      if (navigation.loadEventEnd > 0) {
        recordRendererSpan('navigation.load', navigation.loadEventEnd);
      }
    };
    if (document.readyState === 'complete') {
      report();
      return;
    }
    const afterLoad = (): void => { setTimeout(report, 0); };
    window.addEventListener('load', afterLoad, { once: true });
    return () => window.removeEventListener('load', afterLoad);
  }, []);
  return null;
}
