'use client';

import type { FormEvent, ReactNode } from 'react';
import { Plus, UserSearch } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

export function FollowPlayerForm({ query, onQueryChange, onSubmit, disabled, busy }: {
  query: string;
  onQueryChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  disabled: boolean;
  busy: boolean;
}): ReactNode {
  return (
    <form onSubmit={onSubmit} className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
      <div className="relative min-w-0 flex-1 sm:w-64">
        {/* Not the magnifier of the list filter below: this field finds a new FACEIT player to follow. */}
        <UserSearch aria-hidden className="pointer-events-none absolute top-3.5 left-3.5 size-4 text-fg-3" />
        <Input value={query} onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Nick o URL de FACEIT" aria-label="Nick o URL de FACEIT"
          className="pl-10" disabled={disabled || busy} autoComplete="off" />
      </div>
      <Button type="submit" disabled={disabled || busy || query.trim() === ''}
        loading={busy} loadingText="Siguiendo…"
        // The kit's disabled recipe (secondary fill, fg-3, 50% opacity) all but
        // vanished on this header; keep it flat and inert but still readable.
        className="shadow-none disabled:border disabled:border-border-strong disabled:bg-surface-2 disabled:text-fg-3 disabled:opacity-100">
        <Plus aria-hidden className="size-4" /> Seguir jugador
      </Button>
    </form>
  );
}
