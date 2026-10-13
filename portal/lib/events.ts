import { db, type Executor } from "../db/client.ts";
import { events } from "../db/schema.ts";
import type { EventType } from "./job-types.ts";

const MAX_DETAIL_CHARS = 4000;

export interface EventInput {
  type: EventType;
  // "system", "worker:<id>", "admin:<userId>" or "user:<userId>".
  actor: string;
  at: number;
  requestId?: string | null;
  workerId?: string | null;
  subjectUserId?: string | null;
  detail?: string | null;
}

// Pass the transaction as executor so the event commits with the change it describes.
export async function recordEvent(input: EventInput, executor: Executor = db): Promise<void> {
  await executor.insert(events).values({
    at: input.at,
    type: input.type,
    actor: input.actor,
    requestId: input.requestId ?? null,
    workerId: input.workerId ?? null,
    subjectUserId: input.subjectUserId ?? null,
    detail: input.detail ? input.detail.slice(0, MAX_DETAIL_CHARS) : null,
  });
}
