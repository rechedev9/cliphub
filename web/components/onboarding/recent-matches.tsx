'use client';

import { useEffect, useState, type ReactElement } from 'react';
import { useRouter } from 'next/navigation';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { Button } from '@/components/ui/button';
import { SteamDownloadDialog } from '@/components/onboarding/steam-download-dialog';
import { loadSteamAccount, type SteamStoredMatch } from '@/lib/api/steam-account';
import { importShareCode } from '@/lib/api/steam-import';
import { PRODUCE_FORMAT, produceHref, type ProduceFormat } from '@/lib/clips/routes';

export function RecentSteamMatches({ format = PRODUCE_FORMAT.short }: { format?: ProduceFormat }): ReactElement | null {
  const router = useRouter();
  const [matches, setMatches] = useState<SteamStoredMatch[] | null>(null);
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<string | undefined>();
  const [downloadCode, setDownloadCode] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const result = await loadSteamAccount();
      if (cancelled) return;
      if (result.kind === 'ok' && result.account.matches.length > 0) {
        setMatches(result.account.matches);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (matches === null) return null;

  async function onDownload(code: string): Promise<void> {
    setPending(code);
    setError(undefined);
    const result = await importShareCode(code);
    setPending(null);
    if (result.kind === 'queued') {
      router.push(produceHref(result.id, format));
      return;
    }
    if (result.kind === 'needCredentials') {
      setDownloadCode(code);
      return;
    }
    setError(result.kind === 'offline' ? 'El servicio local no está en marcha.' : result.message);
  }

  return (
    <section className="studio-panel flex flex-col gap-4 p-4 @[34rem]/content:p-6">
      <SectionEyebrow label="TUS PARTIDAS RECIENTES" />
      <ol className="flex flex-col gap-2">
        {matches.map((match) => (
          <li key={match.shareCode} className="flex flex-col gap-2 border border-border bg-surface-1 px-3 py-3 @[34rem]/content:flex-row @[34rem]/content:items-center">
            <div className="min-w-0 flex-1">
              <p className="font-mono text-body-sm tabular-nums text-fg-1">Partida {match.matchId}</p>
              <p className="truncate font-mono text-meta text-fg-3">{match.shareCode}</p>
            </div>
            <Button
              type="button"
              size="sm"
              loading={pending === match.shareCode}
              loadingText="Descargando…"
              onClick={() => { void onDownload(match.shareCode); }}
            >
              DESCARGAR
            </Button>
          </li>
        ))}
      </ol>
      {error ? <p role="alert" className="text-body-sm text-destructive">{error}</p> : null}
      <SteamDownloadDialog
        open={downloadCode !== null}
        code={downloadCode ?? ''}
        onOpenChange={(open) => { if (!open) setDownloadCode(null); }}
        onQueued={(jobId) => { router.push(produceHref(jobId, format)); }}
      />
    </section>
  );
}
