'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { deleteCommunityRankings, getCommunitySettings, putCommunitySettings, submitCommunityRankings } from '@/lib/api/library';
import type { CommunitySettings as Settings } from '@/lib/api/community';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { SettingsSection } from './settings-section';

type Pending = 'toggle' | 'submit' | 'delete' | null;

const dateTime = new Intl.DateTimeFormat('es-ES', { dateStyle: 'medium', timeStyle: 'short' });

/** Why a submission did not go through, next to the button that sent it. */
function submitErrorText(code: string | undefined): string | null {
  if (code === undefined || code === '') return null;
  if (code === 'steam_id_required') return 'Activado, pero falta tu SteamID: indícalo en la sección de Steam para que Studio sepa qué jugador eres.';
  return 'Activado, pero el servidor de ClipHub no respondió. Studio lo reintentará solo; también puedes enviarlo ahora.';
}

/** Opt-in for the global ranking: off by default, says exactly what leaves the PC, and deletes on request. */
export function CommunitySettings(): ReactNode {
  const [settings, setSettings] = useState<Settings | null | 'failed'>(null);
  const [pending, setPending] = useState<Pending>(null);
  const [message, setMessage] = useState<{ kind: 'error' | 'ok'; text: string } | null>(null);

  useEffect(() => {
    let active = true;
    getCommunitySettings()
      .then((value) => {
        if (active) setSettings(value);
      })
      .catch(() => {
        if (active) setSettings('failed');
      });
    return () => {
      active = false;
    };
  }, []);

  const run = (kind: Exclude<Pending, null>, action: () => Promise<Settings>, done: (value: Settings) => string | null): void => {
    setPending(kind);
    setMessage(null);
    action()
      .then((value) => {
        setSettings(value);
        const text = done(value);
        if (text !== null) setMessage({ kind: value.submit_error ? 'error' : 'ok', text });
      })
      .catch(() => setMessage({ kind: 'error', text: 'El servidor de ClipHub no respondió. Inténtalo de nuevo en unos minutos.' }))
      .finally(() => setPending(null));
  };

  let body: ReactNode;
  if (settings === 'failed') {
    body = <p role="alert" className="text-body-sm text-destructive">No se pudo leer la preferencia. Comprueba que ClipHub Studio sigue abierto.</p>;
  } else if (settings === null) {
    body = (
      <div role="status" aria-label="Leyendo el ranking global" className="flex flex-col gap-3">
        <Skeleton className="h-16 w-full rounded-none" />
        <Skeleton className="h-11 w-[14rem] rounded-none" />
      </div>
    );
  } else {
    const on = settings.rankings_enabled;
    body = (
      <div className="flex flex-col gap-4">
        <p className="text-body-sm text-fg-2">
          Si lo activas, Studio envía una vez al día a cliphub.rechedev.cloud las cifras de tu jugador por mapa (bajas, muertes y daño por ronda, KAST,
          aperturas, tradeos, granadas, flashes y WPA) con sus rondas y partidas, bajo un identificador aleatorio de esta instalación. No envía nombres,
          SteamID, nombres de demos ni rutas de archivos. Tu jugador es el SteamID de la sección de Steam.
        </p>
        <p className="text-body-sm text-fg-1">
          {on ? 'Activado' : 'Desactivado'}
          {on && settings.last_submitted_at ? `. Último envío: ${dateTime.format(Date.parse(settings.last_submitted_at))}.` : ''}
          {!settings.server_enabled ? ' El servidor de ClipHub está desactivado en esta instalación.' : ''}
        </p>
        {message ? (
          <p role={message.kind === 'error' ? 'alert' : 'status'} className={message.kind === 'error' ? 'text-body-sm text-warning' : 'text-body-sm text-fg-2'}>
            {message.text}
          </p>
        ) : null}
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant={on ? 'outline' : 'default'}
            disabled={!settings.server_enabled || pending !== null}
            loading={pending === 'toggle'}
            loadingText="Guardando…"
            onClick={() => run('toggle', () => putCommunitySettings(!on), (value) => (value.rankings_enabled ? submitErrorText(value.submit_error) ?? 'Activado y enviado.' : null))}
          >
            {on ? 'Desactivar ranking global' : 'Activar ranking global'}
          </Button>
          {on ? (
            <Button
              type="button"
              variant="outline"
              disabled={pending !== null}
              loading={pending === 'submit'}
              loadingText="Enviando…"
              onClick={() => run('submit', submitCommunityRankings, () => 'Enviado.')}
            >
              Enviar ahora
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            disabled={!settings.server_enabled || pending !== null}
            loading={pending === 'delete'}
            loadingText="Borrando…"
            onClick={() => run('delete', deleteCommunityRankings, () => 'Borrado del servidor. El ranking global queda desactivado.')}
          >
            Borrar mis datos del servidor
          </Button>
        </div>
      </div>
    );
  }

  return (
    <SettingsSection id="community" title="Ranking global" description="Compara tu jugador con el de otras instalaciones de Studio, sin nombres ni SteamID.">
      {body}
    </SettingsSection>
  );
}
