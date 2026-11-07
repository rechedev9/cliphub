import type { ReactNode } from 'react';

export type SettingsSectionProps = {
  /** Anchor for links such as `/settings#community`; also names the heading. */
  id: string;
  title: string;
  description?: ReactNode;
  /** The section's state, such as "Sin configurar", under the description. */
  status?: ReactNode;
  children: ReactNode;
};

/**
 * One block of Ajustes: what it is on the left, its controls on the right on a
 * wide window, stacked on a narrow one. Sections are split by a hairline, not boxed.
 */
export function SettingsSection({ id, title, description, status, children }: SettingsSectionProps): ReactNode {
  const titleId = `${id}-title`;
  return (
    <section
      id={id}
      aria-labelledby={titleId}
      className="grid scroll-mt-20 gap-4 border-t border-border pt-7 @[56rem]/content:grid-cols-[17rem_minmax(0,1fr)] @[56rem]/content:gap-12"
    >
      <div className="flex flex-col items-start gap-1.5">
        <h2 id={titleId} className="text-body-lg font-semibold text-fg-1">
          {title}
        </h2>
        {description ? <div className="text-body-sm text-fg-2">{description}</div> : null}
        {status ? <div className="mt-1.5">{status}</div> : null}
      </div>
      <div className="flex max-w-2xl min-w-0 flex-col gap-4">{children}</div>
    </section>
  );
}
