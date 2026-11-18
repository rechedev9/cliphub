'use client';

import { useId, useMemo, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Check, ChevronsUpDown, Globe2, Map as MapIcon, Search } from 'lucide-react';
import { filterRankingOptions, sameRankingChoice, type RankingChoice, type RankingOption } from '@/lib/faceit-ranking';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { CountryFlag } from './country-flag';

function OptionMark({ option }: { option: RankingOption }): ReactNode {
  if (option.kind === 'country') return <CountryFlag code={option.code} decorative />;
  const Icon = option.kind === 'region' ? Globe2 : MapIcon;
  return <Icon aria-hidden className="size-4 shrink-0 text-fg-3" />;
}

/** Picks the country, FACEIT region or zone whose ranking the rail lists; typing filters by code or name. */
export function RankingPicker({ options, value, onChange, disabled }: {
  options: readonly RankingOption[];
  value: RankingChoice;
  onChange: (choice: RankingChoice) => void;
  disabled?: boolean;
}): ReactNode {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const listID = useId();
  const shown = useMemo(() => filterRankingOptions(options, query), [options, query]);
  const current = options.find((option) => sameRankingChoice(option, value));

  function openChange(next: boolean): void {
    setOpen(next);
    if (!next) return;
    setQuery('');
    setActive(Math.max(0, options.findIndex((option) => sameRankingChoice(option, value))));
  }

  function choose(option: RankingOption): void {
    onChange({ kind: option.kind, code: option.code });
    setOpen(false);
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>): void {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      const step = event.key === 'ArrowDown' ? 1 : -1;
      setActive((index) => (index + step + shown.length) % Math.max(shown.length, 1));
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const option = shown[active];
      if (option !== undefined) choose(option);
    }
  }

  return (
    <Popover open={open} onOpenChange={openChange}>
      <PopoverTrigger asChild>
        <Button variant="outline" disabled={disabled} className="w-full justify-between normal-case tracking-normal"
          aria-label={`País, región o zona: ${current?.hint || current?.label || value.code.toUpperCase()}`}>
          <span className="flex min-w-0 items-center gap-2">
            {current ? <OptionMark option={current} /> : null}
            <span className="shrink-0 font-semibold text-fg-1">{current?.label ?? value.code.toUpperCase()}</span>
            {current?.hint ? <span className="truncate font-normal text-fg-3">{current.hint}</span> : null}
          </span>
          <ChevronsUpDown aria-hidden className="size-4 shrink-0 text-fg-3" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) min-w-64 p-0">
        <div className="relative border-b border-border p-2">
          <Search aria-hidden className="pointer-events-none absolute top-1/2 left-5 size-4 -translate-y-1/2 text-fg-3" />
          <Input role="combobox" aria-expanded aria-controls={listID} aria-autocomplete="list"
            aria-activedescendant={shown[active] === undefined ? undefined : `${listID}-${active}`}
            aria-label="Buscar país, región o zona" placeholder="Buscar país, región o zona" value={query} autoComplete="off"
            onChange={(event) => { setQuery(event.target.value); setActive(0); }} onKeyDown={onKeyDown} className="pl-9" />
        </div>
        <ul id={listID} role="listbox" aria-label="Países, regiones y zonas" className="max-h-72 overflow-y-auto overscroll-contain p-1">
          {shown.map((option, index) => {
            const selected = sameRankingChoice(option, value);
            return (
              <li key={`${option.kind}:${option.code}`} id={`${listID}-${index}`} role="option" aria-selected={selected}
                ref={index === active ? (node) => { node?.scrollIntoView({ block: 'nearest' }); } : undefined}
                onPointerMove={() => setActive(index)} onClick={() => choose(option)}
                className={cn('flex min-h-9 cursor-pointer items-center gap-2 rounded-sm px-2 text-body-sm', index === active && 'bg-surface-3')}>
                <OptionMark option={option} />
                <span className="shrink-0 font-semibold text-fg-1">{option.label}</span>
                <span className="min-w-0 flex-1 truncate text-fg-3">{option.hint}</span>
                {selected ? <Check aria-hidden className="size-4 shrink-0 text-primary" /> : null}
              </li>
            );
          })}
        </ul>
        {shown.length === 0 ? <p role="status" className="px-3 pb-3 text-body-sm text-fg-2">Ningún país, región o zona coincide.</p> : null}
      </PopoverContent>
    </Popover>
  );
}
