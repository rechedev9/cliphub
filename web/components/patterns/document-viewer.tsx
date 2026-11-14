'use client';

import type { ReactNode } from 'react';
import { Download, ExternalLink } from 'lucide-react';
import { formatShortDate } from '@/lib/format';
import { formatDocumentSize, teamDocumentContentHref, TEAM_DOCUMENT_SANDBOX, type TeamDocument } from '@/lib/api/team-documents';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

/**
 * A team document full size. The iframe and the response both sandbox it without the app's origin,
 * so its scripts run (charts, tabs) but never read the session or call the API.
 */
export function DocumentViewer({ document, onClose }: { document: TeamDocument | null; onClose: () => void }): ReactNode {
  const href = document === null ? '' : teamDocumentContentHref(document.id);
  return (
    <Dialog open={document !== null} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className="flex h-[min(92dvh,64rem)] max-w-[calc(100%-1rem)] flex-col gap-0 p-0 sm:max-w-[min(96vw,90rem)]">
        <DialogHeader className="flex-row flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-border px-4 py-3 pr-12 text-left">
          <div className="flex min-w-0 flex-col gap-0.5">
            <DialogTitle className="truncate">{document?.title}</DialogTitle>
            <DialogDescription className="truncate">
              {document === null ? null : `${document.team_name} · ${formatShortDate(document.updated_at)} · ${formatDocumentSize(document.size_bytes)}`}
            </DialogDescription>
          </div>
          <div className="flex shrink-0 gap-1.5">
            <Button asChild variant="outline" size="sm">
              <a href={href} target="_blank" rel="noopener noreferrer">
                <ExternalLink aria-hidden />
                Pestaña nueva
              </a>
            </Button>
            <Button asChild variant="outline" size="sm">
              <a href={href} download={document?.file_name}>
                <Download aria-hidden />
                Descargar
              </a>
            </Button>
          </div>
        </DialogHeader>
        {document === null ? null : (
          <iframe
            key={document.id}
            src={href}
            title={document.title}
            sandbox={TEAM_DOCUMENT_SANDBOX}
            referrerPolicy="no-referrer"
            className="min-h-0 w-full flex-1 rounded-b-lg border-0 bg-white"
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
