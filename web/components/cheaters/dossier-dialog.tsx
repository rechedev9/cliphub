'use client';

import { useState, type ReactNode } from 'react';
import { Check, Copy, ExternalLink } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { VerdictBadge } from '@/components/cheaters/verdict-badge';
import type { AnticheatDossier } from '@/lib/api/anticheat';
import { writeClipboardText } from '@/lib/clipboard-write';

/**
 * The report kit for one player.
 *
 * This dialog deliberately stops at "here is the evidence and here is where
 * the official flow lives". ClipHub does not submit reports and does not
 * help produce several of them against one account: Valve decides cheating
 * bans from its own detection, not from report volume, and coordinated mass
 * reporting is both ineffective and against the Steam Subscriber Agreement.
 * The dossier's own policy block says exactly that, and it is rendered here
 * rather than hidden behind a link.
 */
export function DossierDialog({
  dossier,
  onClose,
}: {
  dossier: AnticheatDossier | null;
  onClose: () => void;
}): ReactNode {
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);

  // Clipboard access is denied outside a secure context and can be refused by
  // the user, so a rejection has to become a visible state rather than an
  // unhandled promise and a button that silently does nothing.
  const copy = async () => {
    if (!dossier) return;
    try {
      await writeClipboardText(dossier.markdown);
    } catch {
      setCopyFailed(true);
      return;
    }
    setCopyFailed(false);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog
      open={dossier !== null}
      onOpenChange={(open) => {
        if (!open) {
          setCopied(false);
          setCopyFailed(false);
          onClose();
        }
      }}
    >
      <DialogContent className="max-h-[85vh] gap-0 overflow-hidden p-0 sm:max-w-3xl">
        {dossier === null ? null : (
          <>
            <DialogHeader className="gap-2 border-b border-border/70 px-6 py-5">
              <div className="flex flex-wrap items-center gap-3">
                <DialogTitle className="font-[family-name:var(--font-display)] text-xl font-bold uppercase tracking-tight">
                  Expediente · {dossier.name || dossier.steamid64}
                </DialogTitle>
                <VerdictBadge verdict={dossier.verdict} />
              </div>
              <DialogDescription>{dossier.policy.summary}</DialogDescription>
            </DialogHeader>

            <ScrollArea className="max-h-[55vh]">
              <div className="flex flex-col gap-6 px-6 py-5">
                <section className="flex flex-col gap-2">
                  <h3 className="font-[family-name:var(--font-mono)] text-xs uppercase tracking-[0.16em] text-muted-foreground">
                    Antes de denunciar
                  </h3>
                  <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm leading-6 text-muted-foreground">
                    {dossier.policy.rules.map((rule) => (
                      <li key={rule}>{rule}</li>
                    ))}
                  </ul>
                  <p className="text-sm leading-6 text-foreground/80">{dossier.policy.rejected}</p>
                </section>

                <section className="flex flex-col gap-3">
                  <h3 className="font-[family-name:var(--font-mono)] text-xs uppercase tracking-[0.16em] text-muted-foreground">
                    Dónde denunciar
                  </h3>
                  {dossier.channels.map((channel) => (
                    <div key={channel.id} className="studio-panel flex flex-col gap-2 rounded-lg px-4 py-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-foreground">{channel.label}</span>
                        {channel.effective ? (
                          <span className="font-[family-name:var(--font-mono)] text-[10px] uppercase tracking-[0.12em] text-primary">
                            vía efectiva
                          </span>
                        ) : null}
                      </div>
                      <p className="text-sm leading-6 text-muted-foreground">{channel.instructions}</p>
                      {channel.url ? (
                        <a
                          href={channel.url}
                          target="_blank"
                          rel="noreferrer noopener"
                          className="inline-flex w-fit items-center gap-1.5 text-sm text-primary underline-offset-4 hover:underline"
                        >
                          {channel.url}
                          <ExternalLink className="size-3.5" aria-hidden />
                        </a>
                      ) : null}
                    </div>
                  ))}
                </section>

                <section className="flex flex-col gap-2">
                  <h3 className="font-[family-name:var(--font-mono)] text-xs uppercase tracking-[0.16em] text-muted-foreground">
                    Evidencia
                  </h3>
                  <pre className="max-w-full overflow-x-auto whitespace-pre-wrap break-words rounded-lg border border-border/70 bg-background/60 p-4 font-[family-name:var(--font-mono)] text-xs leading-5 text-muted-foreground">
                    {dossier.markdown}
                  </pre>
                </section>
              </div>
            </ScrollArea>

            <div className="flex flex-wrap items-center justify-end gap-3 border-t border-border/70 px-6 py-4">
              {copyFailed ? (
                <span role="alert" className="mr-auto text-sm text-destructive">
                  No se pudo copiar al portapapeles; selecciona el texto del expediente y cópialo a mano.
                </span>
              ) : null}
              <Button variant="outline" onClick={() => void copy()}>
                {copied ? <Check aria-hidden /> : <Copy aria-hidden />}
                {copied ? 'COPIADO' : 'COPIAR EXPEDIENTE'}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
