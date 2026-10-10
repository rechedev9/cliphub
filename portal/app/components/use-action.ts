"use client";

import { useCallback, useState } from "react";

import { actionErrorText } from "./labels";
import { postAction } from "./post-action";
import type { ActionRequest, ActionResult } from "./post-action";

export interface ActionRunner {
  // The key of the action in flight, so its button can show it and the others disable.
  busyKey: string | null;
  error: string | null;
  run: (key: string, request: ActionRequest) => Promise<ActionResult>;
  clearError: () => void;
}

// Runs one mutation at a time and refreshes the view after it, also on failure.
export function useAction(onSettled: () => void): ActionRunner {
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(
    async (key: string, request: ActionRequest) => {
      setBusyKey(key);
      setError(null);
      const result = await postAction(request);
      if (!result.ok) setError(actionErrorText(result.code));
      setBusyKey(null);
      onSettled();
      return result;
    },
    [onSettled],
  );

  const clearError = useCallback(() => setError(null), []);
  return { busyKey, error, run, clearError };
}
