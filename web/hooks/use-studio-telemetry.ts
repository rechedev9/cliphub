'use client';

import { useEffect, useState } from 'react';
import { getDesktopSettingsBridge, type StudioTelemetryStatus } from '@/lib/desktop-settings';

// Support code and session are fixed for a launch, so every diagnostic block on
// a page shares one IPC read. Consent can change: callers that decide what to
// promise the user ask for a fresh read.
let shared: Promise<StudioTelemetryStatus | null> | null = null;

function readStatus(): Promise<StudioTelemetryStatus | null> {
  const bridge = getDesktopSettingsBridge();
  if (bridge === null) return Promise.resolve(null);
  return bridge.getTelemetry().catch(() => null);
}

/** Desktop diagnostics status; null in a plain browser or while loading. */
export function useStudioTelemetry({ fresh = false, enabled = true }: { fresh?: boolean; enabled?: boolean } = {}): StudioTelemetryStatus | null {
  const [status, setStatus] = useState<StudioTelemetryStatus | null>(null);
  useEffect(() => {
    if (!enabled) return;
    let alive = true;
    let read: Promise<StudioTelemetryStatus | null>;
    if (fresh) {
      read = readStatus();
    } else {
      shared ??= readStatus().then((value) => {
        if (value === null) shared = null;
        return value;
      });
      read = shared;
    }
    void read.then((value) => {
      if (alive) setStatus(value);
    });
    return () => {
      alive = false;
    };
  }, [fresh, enabled]);
  return status;
}
