'use client';

import { useState, type ReactNode } from 'react';
import { dismissFirstRunGuide, isFirstRunGuideDismissed } from '@/lib/clips/first-run';
import { firstRunComplete, firstRunProgress, type HubModel } from '@/lib/clips/hub';
import { FirstRunGuide } from '@/components/clips-hub/first-run-guide';

/** Guide on a populated hub: gone once all three steps are done, or once the user hides it. */
export function HubGuide({ model }: { model: Pick<HubModel, 'rows' | 'clips'> }): ReactNode {
  const [dismissed, setDismissed] = useState(isFirstRunGuideDismissed);
  const progress = firstRunProgress(model);
  if (dismissed || firstRunComplete(progress)) return null;
  return (
    <FirstRunGuide
      progress={progress}
      onDismiss={() => {
        dismissFirstRunGuide();
        setDismissed(true);
      }}
    />
  );
}
