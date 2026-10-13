import type { Metadata } from "next";
import { redirect } from "next/navigation";
import {
  AlignRightIcon,
  ArrowRightIcon,
  CrosshairIcon,
  DownloadIcon,
  ListChecksIcon,
  PlusIcon,
} from "lucide-react";

import { auth } from "@/auth";
import { Eyebrow, RecDot } from "@/components/hud";
import { PageShell } from "@/components/page-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { loadUserLimits } from "@/lib/user-limits";
import { cn } from "@/lib/utils";

import { ReelPlayer } from "./reel-player";
import { Reveal } from "./reveal";
import { SiteFooter } from "./site-footer";

const SITE_TITLE = "ClipHub · Tus jugadas de CS2 en vídeo, grabadas en tu PC o en la nube";
const SITE_DESCRIPTION =
  "Crea tus vídeos de CS2 en ClipHub Studio y elige dónde se graban: en tu PC o en la nube de ClipHub. Aquí vinculas Studio, sigues tus trabajos y descargas los resultados.";

// Set here, not in the root layout, so other pages do not inherit the landing's card.
export const metadata: Metadata = {
  title: { absolute: SITE_TITLE },
  description: SITE_DESCRIPTION,
  openGraph: {
    type: "website",
    siteName: "ClipHub",
    locale: "es_ES",
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
    url: "/",
  },
  twitter: {
    card: "summary_large_image",
    title: SITE_TITLE,
    description: SITE_DESCRIPTION,
  },
};

const NAV_LINK = "text-fg-2 transition-colors duration-(--dur-fast) hover:text-fg-1";
const INLINE_LINK =
  "text-primary underline decoration-primary/35 underline-offset-[0.2em] hover:decoration-current";
const SPLIT =
  "grid gap-10 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:items-start lg:gap-18";
const READOUT_ROW = "flex justify-between gap-4 py-1 text-fg-1";
const RULE_ROW =
  "grid grid-cols-[5.5rem_minmax(0,1fr)] items-baseline gap-4 border-b border-border-subtle py-4 text-base leading-[1.55]";
const RULE_LABEL = "font-mono text-meta text-fg-3 uppercase";

const FEATURES = [
  {
    icon: CrosshairIcon,
    title: "Grabado dentro del juego",
    body: "Tu demo se reproduce en CS2 y se graba desde tu punto de vista, con la imagen del propio juego.",
  },
  {
    icon: AlignRightIcon,
    title: "Con el killfeed original",
    body: "Las bajas salen como en la partida: el killfeed de CS2, centrado en tu jugador.",
  },
  {
    icon: ListChecksIcon,
    title: "Con las jugadas que tú eliges",
    body: "En ClipHub Studio abres la demo, marcas las jugadas que quieres y eliges el estilo del vídeo.",
  },
  {
    icon: DownloadIcon,
    title: "Un MP4 listo para subir",
    body: "En vertical, para Shorts, Reels o TikTok. Vuelve solo a tu Studio y también queda en tu panel.",
  },
] as const;

const STEPS = [
  {
    title: "Crea el vídeo en Studio",
    body: "Abres la demo de tu partida en ClipHub Studio, eliges tus jugadas y el estilo del vídeo. Eso no cambia: se hace en tu PC.",
    variant: "outline",
    status: "En Studio",
  },
  {
    title: "Elige dónde se graba",
    body: "Antes de crear el Short decides: en este PC o en la nube de ClipHub. En la nube, Studio sube la demo y te pone en cola. Cuando la demo ya está subida puedes cerrarlo.",
    variant: "stream",
    status: "Grabando",
  },
  {
    title: "Recoge el resultado",
    body: "El vídeo terminado vuelve solo a tu Studio. También queda unos días en tu panel de este portal, por si quieres descargarlo desde otro equipo.",
    variant: "success",
    status: "Listo",
  },
] as const;

const PORTAL_USES = [
  {
    label: "Vincular",
    text: "Studio te da un código de ocho caracteres. Lo confirmas aquí una vez por equipo y queda conectado a tu cuenta.",
  },
  {
    label: "Seguir",
    text: "Tu puesto en la cola, cuándo se calcula que empieza y por dónde va la grabación, aunque Studio esté cerrado.",
  },
  {
    label: "Descargar",
    text: "Cada vídeo terminado tiene su enlace de descarga hasta que caduca. Studio ya los baja por ti; esto es para otro equipo.",
  },
] as const;

const RULES = [
  {
    label: "Cola",
    text: "La nube es un único equipo que graba un vídeo cada vez, así que hay cola.",
  },
  {
    label: "Límite",
    text: "Hay un máximo de trabajos y de minutos de grabación por persona y día.",
  },
  {
    label: "Acceso",
    text: "Una cuenta nueva puede necesitar que aprobemos su acceso a la nube. Mientras tanto puedes grabar en tu PC, que no necesita cuenta.",
  },
  {
    label: "Si falla",
    text: "Si la nube está parada o llena, Studio te lo dice y te deja grabar en tu PC.",
  },
  {
    label: "Descarga",
    text: "Los vídeos terminados se borran de aquí pasados unos días.",
  },
] as const;

const QUESTIONS = [
  {
    question: "¿Necesito ClipHub Studio?",
    answer:
      "Sí. Los vídeos se crean en ClipHub Studio: ahí abres la demo, eliges tus jugadas y el estilo. Este portal no crea vídeos: sirve para vincular Studio con tu cuenta, seguir tus trabajos en la nube y descargar los resultados.",
  },
  {
    question: "¿Qué cambia entre grabar en mi PC y en la nube?",
    answer:
      "En tu PC la grabación la hace tu propio equipo y no necesitas cuenta. En la nube, Studio sube la demo y la graba el equipo de ClipHub: cuando la demo ya está subida puedes cerrar Studio, y el vídeo terminado vuelve solo.",
  },
  {
    question: "¿Cuánto tarda un vídeo en la nube?",
    answer:
      "Depende de la cola, porque se graba un vídeo cada vez. En tu panel ves tu puesto y una estimación de cuándo empieza. Es una estimación y puede moverse según lo que haya delante.",
  },
  {
    question: "¿Qué pasa si la nube está parada o llena?",
    answer:
      "Studio te lo dice y te deja grabar en tu PC. Si ya tenías un trabajo en cola, tu puesto se mantiene hasta que la nube vuelve.",
  },
  {
    question: "¿Hasta cuándo puedo descargar un vídeo?",
    answer:
      "Studio descarga cada vídeo por ti cuando termina. Además queda unos días en tu panel, con la fecha hasta la que está disponible. Después se borra de aquí.",
  },
  {
    question: "¿De dónde saco la demo de mi partida?",
    answer:
      "En CS2, entra en la pestaña Ver, abre Tus partidas y descarga la que quieras. Si jugaste en FACEIT, la demo se descarga desde la sala de la partida. Después la abres en ClipHub Studio.",
  },
] as const;

function Kicker({ children }: { children: React.ReactNode }) {
  return (
    <p className="mb-3.5 flex items-center gap-2.5 font-mono text-meta tracking-[0.18em] text-primary uppercase before:h-px before:w-6 before:bg-current before:opacity-70">
      {children}
    </p>
  );
}

function SectionTitle({ className, children }: { className?: string; children: React.ReactNode }) {
  return (
    <h2
      className={cn(
        "mb-4 text-[clamp(1.85rem,4.2vw,2.75rem)] leading-[1.06] font-bold tracking-[-0.028em] text-balance",
        className,
      )}
    >
      {children}
    </h2>
  );
}

function EnterButton() {
  return (
    <Button asChild size="lg" className="group text-body-lg">
      <a href="/login">
        Entrar en mi panel
        <ArrowRightIcon
          aria-hidden
          className="transition-transform duration-(--dur-base) ease-entrance group-hover:translate-x-0.5"
        />
      </a>
    </Button>
  );
}

function LinkStudioButton() {
  return (
    <Button asChild variant="outline" size="lg" className="text-body-lg">
      <a href="/link">Vincular Studio</a>
    </Button>
  );
}

export default async function Home() {
  const session = await auth();
  if (session?.user) redirect("/dashboard");

  const { maxActive, dailySeconds } = loadUserLimits();

  const facts = [
    { value: "2", label: "Sitios donde grabar: tu PC o la nube" },
    { value: "1", label: "Vídeo grabándose cada vez en la nube" },
    { value: String(maxActive), label: "Trabajos en la nube a la vez por persona" },
    { value: `${Math.round(dailySeconds / 60)} min`, label: "De grabación en la nube al día" },
  ];

  return (
    <PageShell width="wide" className="overflow-x-clip pt-5">
      <div className="flex items-center justify-between gap-4">
        <a
          className="inline-flex items-center gap-2.5 text-[1.125rem] font-bold tracking-wide text-fg-1"
          href="/"
          aria-label="ClipHub, inicio"
        >
          <img src="/brand/cliphub-mark.svg" alt="" width={32} height={32} className="size-8" />
          <span>
            Clip<span className="text-primary">Hub</span>
          </span>
        </a>
        <nav className="hidden gap-8 md:flex" aria-label="Secciones">
          <a className={NAV_LINK} href="#como-funciona">
            Cómo funciona
          </a>
          <a className={NAV_LINK} href="#portal">
            El portal
          </a>
          <a className={NAV_LINK} href="#preguntas">
            Preguntas
          </a>
        </nav>
      </div>

      <section className="relative isolate grid items-center gap-14 pt-14 pb-18 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)] lg:gap-8 lg:pt-20 lg:pb-24">
        <div aria-hidden="true" className="hud-grid pointer-events-none absolute inset-0 -z-10 opacity-45" />

        <div className="motion-safe:*:animate-rise motion-safe:*:nth-2:[animation-delay:60ms] motion-safe:*:nth-3:[animation-delay:120ms] motion-safe:*:nth-4:[animation-delay:180ms] motion-safe:*:nth-5:[animation-delay:240ms]">
          <Eyebrow>CS2 · captura en la nube</Eyebrow>
          <h1 className="mb-6 text-[clamp(2.4rem,6vw,4.1rem)] leading-none font-bold tracking-[-0.035em] text-balance">
            Tus jugadas en vídeo, grabadas en tu PC o en la{" "}
            <span className="text-primary [text-shadow:0_0_2.5rem_color-mix(in_oklch,var(--primary)_35%,transparent)]">
              nube de ClipHub
            </span>
            .
          </h1>
          <p className="mb-8 max-w-[46ch] text-[1.1875rem] leading-[1.6] text-pretty text-fg-2">
            Los vídeos se crean en ClipHub Studio. Tú eliges dónde se graba cada uno: en tu PC o en nuestra nube. En
            este portal vinculas Studio con tu cuenta, sigues tus trabajos en la nube y descargas los resultados.
          </p>
          <div className="flex flex-wrap items-center gap-3">
            <EnterButton />
            <LinkStudioButton />
          </div>
          <p className="mt-5 text-body-sm text-fg-3">Entras con Google o Discord.</p>
        </div>

        <div className="relative w-full max-w-[19rem] justify-self-center motion-safe:animate-rise motion-safe:[animation-delay:180ms] lg:max-w-[20rem]">
          <div
            aria-hidden="true"
            className="pointer-events-none absolute inset-x-[-6%] inset-y-[6%] -z-10 rounded-full bg-[radial-gradient(closest-side,color-mix(in_oklch,var(--primary)_30%,transparent),transparent)] blur-[2.5rem]"
          />

          {/* A surface ring then a hairline: the frame reads as a device bezel. */}
          <div className="relative aspect-[9/16] overflow-hidden rounded-[1.125rem] border border-border-strong bg-surface-0 shadow-[0_0_0_0.375rem_var(--surface-2),0_0_0_calc(0.375rem+1px)_var(--border),0_2.5rem_4rem_-1.5rem_oklch(0.03_0.02_264/0.9)]">
            <ReelPlayer />
            {/* Shade only the very top and the bottom: the killfeed stays untouched. */}
            <div
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_bottom,color-mix(in_oklch,var(--surface-0)_50%,transparent),transparent_12%,transparent_60%,color-mix(in_oklch,var(--surface-0)_75%,transparent))]"
            />
            <Badge
              variant="outline"
              className="absolute top-3 left-3 z-10 gap-1.5 border-border bg-surface-0/80 px-2.5 py-1 font-mono text-fg-1 uppercase backdrop-blur-sm"
            >
              <RecDot />
              Muestra real
            </Badge>
          </div>
          <span
            aria-hidden="true"
            className="pointer-events-none absolute -top-4 -left-4 size-5 border-t-2 border-l-2 border-primary"
          />
          <span
            aria-hidden="true"
            className="pointer-events-none absolute -right-4 -bottom-4 size-5 border-r-2 border-b-2 border-primary"
          />

          {/* A cloud job in the product's own HUD language. Decorative. */}
          <div
            aria-hidden="true"
            className="absolute inset-x-3 bottom-3 rounded-lg border border-border bg-surface-2 px-3.5 py-3 font-mono text-[0.8125rem] leading-[1.6] shadow-lg lg:right-auto lg:bottom-9 lg:-left-22 lg:w-[16.5rem] lg:px-4.5 lg:py-4 lg:shadow-[var(--elev-bevel),0_1.5rem_3rem_-1rem_oklch(0.03_0.02_264/0.9)]"
          >
            <div className="mb-2 flex items-center gap-2 border-b border-border-subtle pb-2.5 text-meta text-fg-3 uppercase">
              <RecDot className="motion-safe:animate-rec-pulse" />
              nube cliphub
            </div>
            <div className={`${READOUT_ROW} motion-safe:animate-readout-1`}>
              <span>mirage.dem</span>
              <span className="text-fg-3">subida</span>
            </div>
            <div className={`${READOUT_ROW} motion-safe:animate-readout-2`}>
              <span>grabando en la nube</span>
              <span className="text-fg-3 tabular-nums">1 / 1</span>
            </div>
            <div className="mt-0.5 mb-1 h-0.5 overflow-hidden rounded-full bg-surface-4">
              <span className="block h-full origin-left bg-primary motion-safe:animate-readout-bar" />
            </div>
            <div className={`${READOUT_ROW} motion-safe:animate-readout-3`}>
              <span>short-01.mp4</span>
              <span className="text-success">listo</span>
            </div>
          </div>
        </div>
      </section>

      <dl className="grid grid-cols-2 border-y border-border-subtle md:grid-cols-4">
        {facts.map((fact) => (
          <div
            key={fact.label}
            className="flex flex-col-reverse justify-end gap-2 border-border-subtle py-6 pr-4 nth-[n+3]:border-t md:border-l md:px-6 md:py-7 md:first:border-l-0 md:first:pl-0 md:nth-[n+3]:border-t-0"
          >
            <dt className="text-body-sm text-fg-2">{fact.label}</dt>
            <dd className="text-[clamp(1.75rem,4vw,2.25rem)] leading-none font-bold tracking-[-0.02em] tabular-nums">
              {fact.value}
            </dd>
          </div>
        ))}
      </dl>

      <section id="resultado" className="mt-22 scroll-mt-8 lg:mt-28">
        <Reveal className={SPLIT}>
          <div>
            <Kicker>Qué recibes</Kicker>
            <SectionTitle>Tu partida, vuelta a grabar desde dentro.</SectionTitle>
            <p className="max-w-[52ch] text-body-lg leading-[1.65] text-pretty text-fg-2">
              La demo guarda la partida entera. ClipHub la reproduce en CS2, graba tus jugadas desde tu punto de
              vista y las monta en un vídeo. Nada de capturas de pantalla ni de plantillas.
            </p>
          </div>
          {/* A ruled grid: the 1px gap shows the container colour as hairlines. */}
          <ul className="grid gap-px overflow-hidden rounded-lg border border-border-subtle bg-border-subtle sm:grid-cols-2">
            {FEATURES.map(({ icon: Icon, title, body }) => (
              <li
                key={title}
                className="bg-surface-1 px-6 pt-6.5 pb-7 transition-colors duration-(--dur-base) hover:bg-surface-2"
              >
                <Icon aria-hidden className="mb-4.5 size-6 text-primary" strokeWidth={1.5} />
                <h3 className="mb-2 text-[1.125rem] leading-[1.15] font-semibold tracking-[-0.01em]">
                  {title}
                </h3>
                <p className="text-fg-2">{body}</p>
              </li>
            ))}
          </ul>
        </Reveal>
      </section>

      <section id="como-funciona" className="mt-22 scroll-mt-8 lg:mt-28">
        <Reveal>
          <Kicker>Cómo funciona</Kicker>
          <SectionTitle>Tres pasos, y tú eliges dónde se graba.</SectionTitle>
          {/* A rail with one node per step: vertical on phones, horizontal above. */}
          <ol className="mt-10 grid md:mt-12 md:grid-cols-3">
            {STEPS.map((step, index) => (
              <li
                key={step.title}
                className="relative flex flex-col items-start border-l border-border pb-9 pl-7 before:absolute before:top-[0.2rem] before:left-[-0.3125rem] before:size-[0.5625rem] before:rounded-full before:bg-primary before:shadow-[0_0_0_4px_color-mix(in_oklch,var(--primary)_16%,transparent)] last:pb-0 md:border-t md:border-l-0 md:pt-7 md:pr-8 md:pb-0 md:pl-0 md:before:top-[-0.3125rem] md:before:left-0"
              >
                <span className="font-mono text-[0.8125rem] tracking-[0.14em] text-primary">
                  {String(index + 1).padStart(2, "0")}
                </span>
                <h3 className="my-2 text-title font-semibold">{step.title}</h3>
                <p className="mb-4.5 max-w-[36ch] text-base leading-[1.6] text-fg-2">{step.body}</p>
                <Badge variant={step.variant} className="mt-auto font-mono uppercase">
                  {step.status}
                </Badge>
              </li>
            ))}
          </ol>
        </Reveal>
      </section>

      <section id="portal" className="mt-22 scroll-mt-8 lg:mt-28">
        <Reveal className={SPLIT}>
          <div>
            <Kicker>El portal</Kicker>
            <SectionTitle>Qué haces en este portal.</SectionTitle>
            <p className="max-w-[52ch] text-body-lg leading-[1.65] text-pretty text-fg-2">
              Aquí no se crean vídeos: eso es cosa de Studio. El portal es la parte de la nube que puedes abrir
              desde cualquier equipo, también desde el móvil.
            </p>
          </div>
          <ul className="border-t border-border-subtle">
            {PORTAL_USES.map((use) => (
              <li key={use.label} className={RULE_ROW}>
                <span className={RULE_LABEL}>{use.label}</span>
                <span>{use.text}</span>
              </li>
            ))}
          </ul>
        </Reveal>
      </section>

      <section id="condiciones" className="mt-22 scroll-mt-8 lg:mt-28">
        <Reveal className={SPLIT}>
          <div>
            <Kicker>Antes de empezar</Kicker>
            <SectionTitle>La nube es un solo equipo.</SectionTitle>
            <p className="mt-6 max-w-[34ch] text-[1.1875rem] leading-[1.55] text-pretty text-fg-1">
              Graba un vídeo cada vez, así que a ratos hay cola. Tu PC sigue ahí: grabar en él no necesita cuenta
              ni espera a nadie.
            </p>
          </div>
          <ul className="border-t border-border-subtle">
            {RULES.map((rule) => (
              <li key={rule.label} className={RULE_ROW}>
                <span className={RULE_LABEL}>{rule.label}</span>
                <span>{rule.text}</span>
              </li>
            ))}
          </ul>
        </Reveal>
      </section>

      <section id="preguntas" className="mt-22 scroll-mt-8 lg:mt-28">
        <Reveal className={SPLIT}>
          <div>
            <Kicker>Preguntas</Kicker>
            <SectionTitle>Lo que conviene saber antes.</SectionTitle>
            <p className="max-w-[52ch] text-body-lg leading-[1.65] text-pretty text-fg-2">
              El detalle completo está en las{" "}
              <a className={INLINE_LINK} href="/terms">
                condiciones
              </a>{" "}
              y en la{" "}
              <a className={INLINE_LINK} href="/privacy">
                política de privacidad
              </a>
              .
            </p>
          </div>
          {/* Native details: the answers are in the HTML, with or without script. */}
          <div className="border-t border-border-subtle">
            {QUESTIONS.map(({ question, answer }) => (
              <details key={question} className="group border-b border-border-subtle">
                <summary className="flex cursor-pointer list-none items-center justify-between gap-6 rounded-sm py-4.5 text-body-lg font-semibold text-fg-1 transition-colors duration-(--dur-fast) hover:text-primary [&::-webkit-details-marker]:hidden">
                  {question}
                  <PlusIcon
                    aria-hidden
                    className="size-4 shrink-0 text-primary transition-transform duration-(--dur-base) ease-entrance group-open:rotate-45"
                  />
                </summary>
                <p className="max-w-[60ch] pb-5 text-base leading-[1.65] text-fg-2">{answer}</p>
              </details>
            ))}
          </div>
        </Reveal>
      </section>

      <Reveal>
        <section className="mt-24 rounded-[1.25rem] border border-border bg-surface-2 bg-[radial-gradient(34rem_16rem_at_50%_0%,color-mix(in_oklch,var(--primary)_16%,transparent),transparent_70%)] px-6 py-14 text-center shadow-lg">
          <SectionTitle className="mx-auto max-w-[18ch]">¿Ya tienes ClipHub Studio?</SectionTitle>
          <p className="mx-auto mb-8 max-w-[40ch] text-body-lg leading-[1.6] text-fg-2">
            Entra con tu cuenta, vincula Studio y elige la nube la próxima vez que crees un vídeo.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <EnterButton />
            <LinkStudioButton />
          </div>
        </section>
      </Reveal>

      <SiteFooter />
    </PageShell>
  );
}
