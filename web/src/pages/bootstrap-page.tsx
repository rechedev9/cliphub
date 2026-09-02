import { ShieldCheck } from 'lucide-react';
import { useSearchParams } from 'react-router-dom';
import type { ReactElement } from 'react';
import { AutomaticBootstrap } from '@/components/bootstrap/automatic-bootstrap';
import { Wordmark } from '@/components/brand/wordmark';
import { SectionEyebrow } from '@/components/brand/section-eyebrow';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

const BOOTSTRAP_ERRORS = {
  capability: 'La capacidad no coincide con la que muestra Local Studio. Cópiala otra vez tal cual.',
  unavailable: 'El servicio local no está respondiendo. Arranca Local Studio y vuelve a intentarlo.',
} as const;

export default function BootstrapPage(): ReactElement {
  const [params] = useSearchParams();
  const code = params.get('error');
  const message = code === null ? null : BOOTSTRAP_ERRORS[code === 'unavailable' ? 'unavailable' : 'capability'];
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-8 px-6 py-12">
      <Wordmark />
      <form id="local-bootstrap-form" action="/api/session/bootstrap" method="post" className="studio-panel studio-panel-raised flex w-full max-w-[30rem] flex-col gap-5 p-7">
        <AutomaticBootstrap formId="local-bootstrap-form" inputId="capability" />
        <div className="flex flex-col gap-3">
          <SectionEyebrow label="Sesión local" />
          <h1 className="font-display text-title font-bold text-fg-1">Autoriza este navegador</h1>
          <p className="text-body text-fg-2">Introduce la capacidad que muestra Local Studio. Se guarda solo como una cookie HttpOnly de esta sesión, nunca sale de tu equipo.</p>
        </div>
        {message === null ? null : <p role="alert" className="rounded-md border border-destructive/45 bg-destructive/10 px-3.5 py-2.5 text-body-sm text-destructive">{message}</p>}
        <div className="flex flex-col gap-2">
          <label htmlFor="capability" className="font-display text-label font-semibold tracking-wide text-fg-1 uppercase">Capacidad local</label>
          <Input id="capability" name="capability" type="password" autoComplete="off" required aria-invalid={message === null ? undefined : true} className="font-mono tracking-wider" />
        </div>
        <Button type="submit" size="lg">Abrir ClipHub</Button>
        <p className="flex items-start gap-2 text-body-sm text-fg-3"><ShieldCheck className="mt-0.5 size-4 shrink-0" aria-hidden />ClipHub no tiene backend alojado: esta pantalla solo autoriza al navegador a hablar con el orquestador que ya corre en este PC.</p>
      </form>
    </main>
  );
}
