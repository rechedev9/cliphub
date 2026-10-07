/** Copy for the toast shown after a long video is queued. */
const QUEUE_HINT = 'Sigue el progreso en Demos y vídeos.';
const QUEUE_BUSY = 'Empezará cuando quede libre CS2.';

/**
 * A plan warning has to be read after create leaves the prep page. The default
 * toast (4s) closes before that sentence is readable; this one stays up and
 * can still be dismissed.
 */
const WARNING_DURATION_MS = 12_000;

export type FullDemoQueueToast = {
  description: string;
  duration?: number;
  closeButton?: boolean;
};

/** Toast options for a queued long video. Plan warnings ride on the description. */
export function fullDemoQueueToast(recBusy: boolean, warnings: readonly { message: string }[] | null | undefined): FullDemoQueueToast {
  const hint = recBusy ? QUEUE_BUSY : QUEUE_HINT;
  const messages = (warnings ?? []).map((warning) => warning.message.trim()).filter((message) => message !== '');
  if (messages.length === 0) return { description: hint };
  return { description: `${messages.join(' ')} ${hint}`, duration: WARNING_DURATION_MS, closeButton: true };
}
