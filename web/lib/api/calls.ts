import { jsonInit, readAll, request } from './library.ts';
import type { LibretaStatus } from './libreta.ts';
import type { TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of the Libreta calls (`library.Call` in Go): one word
 * of the team's vocabulary, what is said and what it means.
 */

export const CALL_SIDES = ['T', 'CT', 'both'] as const;
export type CallSide = (typeof CALL_SIDES)[number];

export function isCallSide(value: string): value is CallSide {
  return CALL_SIDES.some((side) => side === value);
}

/** The server's caps (`library.MaxCallNameRunes`, `library.MaxCallMeaningRunes`). */
export const MAX_CALL_NAME = 60;
export const MAX_CALL_MEANING = 1000;

export type LibraryCall = {
  id: string;
  name: string;
  side: CallSide;
  /** What the call asks the team to do; may hold newlines. */
  meaning: string;
  status: LibretaStatus;
  tags: string[];
  created_at: string;
  updated_at: string;
};

/** A new call: side defaults to both and status to active on the server. */
export type CallInput = {
  name: string;
  side?: CallSide;
  meaning?: string;
  status?: LibretaStatus;
  tags?: string[];
};

export type CallPatch = Partial<CallInput>;

export const CALL_KEYS = [
  'id', 'name', 'side', 'meaning', 'status', 'tags', 'created_at', 'updated_at',
] as const satisfies readonly (keyof LibraryCall)[];
export const CALL_FIELDS = ['name', 'side', 'meaning', 'status', 'tags'] as const satisfies readonly (keyof CallInput)[];
export const CALL_LIST_QUERY = ['side', 'status', 'tag', 'include_archived'] as const;

export type CallListQuery = {
  /** A side also returns the calls of both sides. */
  side?: TacticalSide;
  status?: LibretaStatus;
  tag?: string;
  /** Archived calls are left out unless this is true or a status is asked for. */
  include_archived?: boolean;
};

/** The Libreta's calls, by name. */
export function listLibraryCalls(query: CallListQuery = {}): Promise<LibraryCall[]> {
  const params = new URLSearchParams();
  if (query.side) params.set('side', query.side);
  if (query.status) params.set('status', query.status);
  if (query.tag) params.set('tag', query.tag);
  if (query.include_archived) params.set('include_archived', 'true');
  return readAll<LibraryCall>('/calls', 'calls', params);
}

export function getLibraryCall(callId: string): Promise<LibraryCall> {
  return request<LibraryCall>(`/calls/${encodeURIComponent(callId)}`);
}

export function createLibraryCall(input: CallInput): Promise<LibraryCall> {
  return request<LibraryCall>('/calls', jsonInit('POST', input));
}

export function patchLibraryCall(callId: string, patch: CallPatch): Promise<LibraryCall> {
  return request<LibraryCall>(`/calls/${encodeURIComponent(callId)}`, jsonInit('PATCH', patch));
}

export function deleteLibraryCall(callId: string): Promise<void> {
  return request<void>(`/calls/${encodeURIComponent(callId)}`, { method: 'DELETE' });
}
