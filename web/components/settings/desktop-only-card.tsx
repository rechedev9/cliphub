import type { ReactNode } from 'react';
import { MonitorCog } from 'lucide-react';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { IconTile } from '@/components/studio/icon-tile';
import { StatusTag } from '@/components/studio/status-tag';

export type DesktopOnlyCardProps = {
  titleId: string;
  title: string;
  children: ReactNode;
  /**
   * What the desktop build actually does with this feature. Rendered as a spec
   * list: the state reads as a capability that lives elsewhere rather than as
   * an apology for a missing browser fallback.
   */
  capabilities?: readonly string[];
};

/**
 * The shared "this feature lives in the desktop app" panel for /settings. Each
 * desktop-only feature keeps its own descriptive title (so two cards on the page
 * stay distinguishable) over a common eyebrow naming where it is available.
 */
export function DesktopOnlyCard({ titleId, title, children, capabilities }: DesktopOnlyCardProps): ReactNode {
  return (
    <section className="studio-panel flex flex-col gap-5 p-5 sm:p-6" aria-labelledby={titleId}>
      <div className="flex flex-col gap-4 @[34rem]/content:flex-row @[34rem]/content:items-start @[34rem]/content:gap-5">
        <IconTile icon={MonitorCog} size="lg" depth="inset" />
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <SectionEyebrow label="CREDENCIALES" />
            <StatusTag tone="primary">Solo escritorio</StatusTag>
          </div>
          <h2 id={titleId} className="font-display text-title font-bold uppercase text-fg-1">
            {title}
          </h2>
          <p className="text-body text-fg-2">{children}</p>
        </div>
      </div>

      {capabilities && capabilities.length > 0 ? (
        <ul className="flex flex-col gap-2 border-t border-border-subtle pt-4">
          {capabilities.map((point) => (
            <li key={point} className="flex items-start gap-3 font-mono text-meta uppercase tracking-wider text-fg-2">
              <span aria-hidden className="mt-[0.3rem] size-1.5 shrink-0 bg-primary shadow-[0_0_6px_currentColor]" />
              {point}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
