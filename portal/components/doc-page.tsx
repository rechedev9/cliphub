import { SiteFooter } from "@/app/site-footer";
import { PageShell } from "@/components/page-shell";

// Long-form legal copy: the children are plain h2/p/ul, styled from here.
const PROSE = [
  "[&_h2]:mt-9 [&_h2]:mb-3 [&_h2]:border-t [&_h2]:border-border-subtle [&_h2]:pt-5",
  "[&_h2]:text-body-lg [&_h2]:font-semibold [&_h2]:tracking-[-0.01em]",
  "[&_p]:mb-3.5 [&_p]:leading-[1.7] [&_p]:text-fg-2",
  "[&_ul]:mb-3.5 [&_ul]:list-disc [&_ul]:pl-4.5",
  "[&_li]:mb-2 [&_li]:leading-[1.7] [&_li]:text-fg-2",
  "[&_strong]:font-semibold [&_strong]:text-fg-1",
  "[&_a]:text-primary [&_a]:underline [&_a]:decoration-primary/35 [&_a]:underline-offset-[0.2em] [&_a]:hover:decoration-current",
  "[&_code]:rounded-sm [&_code]:border [&_code]:border-border-subtle [&_code]:bg-surface-3 [&_code]:px-1.5 [&_code]:font-mono [&_code]:text-[0.9em]",
].join(" ");

export function DocPage({
  title,
  updated,
  children,
}: {
  title: string;
  updated: string;
  children: React.ReactNode;
}) {
  return (
    <PageShell width="read">
      <h1 className="mb-1.5 text-[2rem] leading-[1.15] font-semibold tracking-[-0.01em]">
        {title}
      </h1>
      <p className="mb-8 font-mono text-meta tracking-[0.08em] text-fg-3 uppercase">{updated}</p>
      <div className={PROSE}>{children}</div>
      <SiteFooter />
    </PageShell>
  );
}
