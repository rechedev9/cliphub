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
  actions,
  className,
}: {
  tabs: readonly RecordTab[];
  defaultTab?: string;
  value?: string;
  onValueChange?: (tab: string) => void;
  /** Buttons that act on the whole record, at the right end of the tab bar. */
  actions?: ReactNode;
  className?: string;
}): ReactNode {
  return (
    <Tabs
      defaultValue={value === undefined ? (defaultTab ?? tabs[0]?.id) : undefined}
      value={value}
      onValueChange={onValueChange}
      className={cn('gap-4', className)}
    >
      {/* Many tabs on a narrow window scroll sideways inside the bar instead of widening the page. */}
      <div className="flex items-end gap-3 border-b border-border">
        <TabsList
          variant="line"
          className="h-auto min-w-0 flex-1 justify-start gap-4 overflow-x-auto overflow-y-hidden rounded-none p-0 [scrollbar-width:thin]"
        >
          {tabs.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id} className="h-10 flex-none rounded-none px-1 after:bottom-0! focus-visible:outline-offset-[-2px]">
              {tab.label}
              {tab.count === undefined ? null : (
                <span className="rounded-sm bg-surface-3 px-1.5 text-meta font-normal text-fg-2">{tab.count}</span>
              )}
            </TabsTrigger>
          ))}
        </TabsList>
        {actions ? <div className="flex shrink-0 items-center gap-2 pb-1">{actions}</div> : null}
      </div>
      {tabs.map((tab) => (
        <TabsContent key={tab.id} value={tab.id}>
          {tab.content}
        </TabsContent>
      ))}
    </Tabs>
  );
}
