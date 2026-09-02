'use client';

import { useEffect } from 'react';
import { bootstrapCapabilityFromHash } from '@/lib/bootstrap-fragment';

interface AutomaticBootstrapProps {
  formId: string;
  inputId: string;
}

/**
 * Local Studio opens /bootstrap#<one-launch-capability>. The fragment never
 * reaches the Go bootstrap endpoint; this component removes it from browser history before
 * submitting the existing POST form. A direct/manual visit simply leaves the
 * recovery form untouched.
 */
export function AutomaticBootstrap({ formId, inputId }: AutomaticBootstrapProps) {
  useEffect(() => {
    const capability = bootstrapCapabilityFromHash(window.location.hash);
    if (capability === null) return;

    window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}`);
    const form = document.getElementById(formId);
    const input = document.getElementById(inputId);
    if (!(form instanceof HTMLFormElement) || !(input instanceof HTMLInputElement)) return;

    input.value = capability;
    form.requestSubmit();
  }, [formId, inputId]);

  return null;
}
