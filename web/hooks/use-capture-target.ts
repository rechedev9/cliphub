'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import type { CaptureStatus } from '@/lib/api/types';
import {
  browserStorage,
  CAPTURE_TARGET,
  loadCaptureTarget,
  resolveCaptureTarget,
  saveCaptureTarget,
  type CaptureTarget,
  type CaptureTargetView,
} from '@/lib/cloud/capture-target';
import { useCloudAccount } from '@/hooks/use-cloud-account';

export type UseCaptureTargetInput = {
  /** Portal job kind; each one remembers its own choice. */
  kind: 'short' | 'full_demo';
  /** CS2 is recording another video on this PC right now. */
  localBusy: boolean;
  initialTarget?: CaptureTarget | null;
  cloudBlocker?: string | null;
};

export type CaptureTargetState = {
  view: CaptureTargetView;
  toCloud: boolean;
  pick: (target: CaptureTarget) => void;
};

/** A producer's "Dónde se graba": this PC's readiness, the cloud account and the choice remembered for the format. */
export function useCaptureTarget({ kind, localBusy, initialTarget = null, cloudBlocker = null }: UseCaptureTargetInput): CaptureTargetState {
  const account = useCloudAccount();
  const [localStatus, setLocalStatus] = useState<CaptureStatus | null>(null);
  const [picked, setPicked] = useState<CaptureTarget | null>(initialTarget);
  const [remembered, setRemembered] = useState<CaptureTarget | null>(null);

  useEffect(() => {
    let mounted = true;
    setRemembered(loadCaptureTarget(browserStorage(), kind));
    api.getCaptureReadiness().then(
      (readiness) => {
        if (mounted) setLocalStatus(readiness.status);
      },
      () => {
        if (mounted) setLocalStatus('offline');
      },
    );
    return () => {
      mounted = false;
    };
  }, [kind]);

  const view = resolveCaptureTarget({ picked, remembered, localStatus, localBusy, account, kind, cloudBlocker });
  function pick(target: CaptureTarget): void {
    setPicked(target);
    saveCaptureTarget({ storage: browserStorage(), kind, target });
  }
  return { view, toCloud: view.target === CAPTURE_TARGET.cloud, pick };
}
