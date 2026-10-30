'use client';

import type { ReactNode } from 'react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';

export type RecordTab = {
  readonly id: string;
  readonly label: string;
  /** Item count shown beside the label, such as the rounds of a partida. */
  readonly count?: number;
  readonly content: ReactNode;
};

/**
 * The sections of a record page as underlined tabs, Salesforce style. Pass
 * `value` and `onValueChange` when a link elsewhere on the page opens a tab.
 */
export function RecordTabs({
  tabs,
  defaultTab,
  value,
  onValueChange,
  className,
}: {
  tabs: readonly RecordTab[];
  defaultTab?: string;
  value?: string;
  onValueChange?: (tab: string) => void;
  className?: string;
}): ReactNode {
  return (
    <Tabs
      defaultValue={value === undefined ? (defaultTab ?? tabs[0]?.id) : undefined}
      value={value}
      onValueChange={onValueChange}
      className={cn('gap-4', className)}
    >
      <TabsList variant="line" className="h-auto w-full justify-start gap-4 rounded-none border-b border-border p-0">
        {tabs.map((tab) => (
          <TabsTrigger key={tab.id} value={tab.id} className="h-10 flex-none rounded-none px-1">
            {tab.label}
            {tab.count === undefined ? null : (
              <span className="rounded-sm bg-surface-3 px-1.5 text-meta font-normal text-fg-2">{tab.count}</span>
            )}
          </TabsTrigger>
        ))}
      </TabsList>
      {tabs.map((tab) => (
        <TabsContent key={tab.id} value={tab.id}>
          {tab.content}
        </TabsContent>
      ))}
    </Tabs>
  );
}
