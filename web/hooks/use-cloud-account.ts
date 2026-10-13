'use client';

import { useSyncExternalStore } from 'react';
import { cloudAccountSnapshot, serverCloudAccountSnapshot, subscribeToCloudAccount } from '@/lib/cloud/account-store';
import type { CloudAccountState } from '@/lib/cloud/parse';

/** The shared cloud account, kept fresh while the component is mounted. */
export function useCloudAccount(): CloudAccountState {
  return useSyncExternalStore(subscribeToCloudAccount, cloudAccountSnapshot, serverCloudAccountSnapshot);
}
