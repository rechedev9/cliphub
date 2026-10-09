import type { Metadata } from "next";
import { redirect } from "next/navigation";
import {
  AlignRightIcon,
  ArrowRightIcon,
  CrosshairIcon,
  DownloadIcon,
  PlusIcon,
  UserCheckIcon,
} from "lucide-react";

import { auth } from "@/auth";
import { Eyebrow, RecDot } from "@/components/hud";
import { PageShell } from "@/components/page-shell";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { loadRequestLimits } from "@/lib/request-limits";
import { loadRetentionConfig } from "@/lib/retention";
import { cn } from "@/lib/utils";

import { ReelPlayer } from "./reel-player";
import { Reveal } from "./reveal";
import { SiteFooter } from "./site-footer";

const SITE_TITLE = "ClipHub · Tus mejores rondas de CS2, montadas a mano";
const SITE_DESCRIPTION =
  "Envía la demo de tu partida de CS2 y recibe un vídeo editado con tus mejores jugadas, grabado dentro del juego. Gratis.";

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

const MEBIBYTE = 1024 * 1024;

const NAV_LINK = "text-fg-2 transition-colors duration-(--dur-fast) hover:text-fg-1";
const INLINE_LINK =
  "text-primary underline decoration-primary/35 underline-offset-[0.2em] hover:decoration-current";
const SPLIT =
  "grid gap-10 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:items-start lg:gap-18";
const READOUT_ROW = "flex justify-between gap-4 py-1 text-fg-1";

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
    icon: UserCheckIcon,
    title: "Elegido por una persona",
    body: "Reviso cada petición y decido qué entra en el montaje. Si dejas una nota, sé dónde mirar.",
  },
  {
    icon: DownloadIcon,
    title: "Un MP4 listo para subir",
    body: "En vertical, para Shorts, Reels o TikTok. Lo descargas desde tu panel.",
  },
] as const;

const QUESTIONS = [
  {
    question: "¿De dónde saco la demo de mi partida?",
    answer:
      "En CS2, entra en la pestaña Ver, abre Tus partidas y descarga la que quieras. Si jugaste en FACEIT, la demo se descarga desde la sala de la partida. Si te llega comprimida (.zst, .bz2 o .gz), descomprímela antes: aquí solo entra el archivo .dem.",
  },
  {
    question: "¿Cuánto tarda en llegar el vídeo?",
    answer:
      "No hay plazos. Depende de la cola y de mi tiempo. Mientras tanto ves el estado de tu petición en tu panel: pendiente de revisión, en proceso o lista.",
  },
  {
    question: "¿Puedo elegir qué jugadas salen?",
    answer:
      "Al subir la demo puedes dejar una nota con quién eres en la partida y qué ronda te interesa. La tengo en cuenta, pero el montaje final lo decido yo.",
  },
  {
    question: "¿Qué pasa con mi demo después?",
    answer:
      "Se usa solo para montar tu vídeo. Pasa del servidor al ordenador donde se graba, y la copia del servidor se borra en cuanto termina esa transferencia. No hay publicidad ni analítica de terceros.",
  },
  {
    question: "¿Sirven las demos de CS:GO?",
    answer:
      "No. Solo se aceptan demos de Counter-Strike 2. Las de CS:GO tienen otro formato y se rechazan al subirlas.",
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

function SendDemoButton() {
  return (
    <Button asChild size="lg" className="group text-body-lg">
      <a href="/dashboard">
        Enviar una demo
        <ArrowRightIcon
          aria-hidden
          className="transition-transform duration-(--dur-base) ease-entrance group-hover:translate-x-0.5"
        />
      </a>
    </Button>
  );
}

export default async function Home() {
  const session = await auth();
  if (session?.user) redirect("/dashboard");

  const { maxActive, maxDaily } = loadRequestLimits();
  const { retentionDays } = loadRetentionConfig();
  const maxDemoMb = Math.floor(
    Number(process.env.MAX_DEMO_BYTES ?? 700 * MEBIBYTE) / MEBIBYTE,
  );

  const steps = [
    {
      title: "Sube tu demo",
      body: "Subes el archivo .dem de la partida. Si quieres, dejas una nota: quién eres y qué ronda te interesa.",
      variant: "outline",
      status: "Pendiente de revisión",
    },
    {
      title: "La reviso a mano",
      body: "Cada petición pasa por mí antes de entrar en la cola. No es una granja de renders: es un ordenador grabando una partida cada vez.",
      variant: "warning",
      status: "En proceso",
    },
    {
      title: "Recibes el vídeo",
      body: `Cuando está montado aparece en tu panel con su enlace de descarga. Tienes ${retentionDays} días para bajártelo.`,
      variant: "success",
      status: "Lista",
    },
  ] as const;

  const facts = [
    { value: "0 €", label: "Gratis y sin publicidad" },
    { value: "1", label: "Partida grabándose cada vez" },
    { value: String(maxActive), label: "Peticiones abiertas por persona" },
    { value: `${retentionDays} días`, label: "Para descargar tu vídeo" },
  ];

  const rules = [
    {
      label: "Cola",
      text: "Se graba una partida cada vez. No hay plazos de entrega.",
    },
    {
      label: "Límite",
      text: `Hasta ${maxActive} peticiones abiertas y ${maxDaily} nuevas al día por persona.`,
    },
    {
      label: "Archivo",
      text: `Solo demos de CS2, en .dem sin comprimir y de hasta ${maxDemoMb} MB.`,
    },
    {
      label: "Revisión",
      text: "Cada demo pasa por mí y puedo rechazarla sin dar motivo.",
    },
    {
      label: "Descarga",
      text: `El vídeo queda ${retentionDays} días en tu panel. Después se borra.`,
    },
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
          <a className={NAV_LINK} href="#resultado">
            Qué recibes
          </a>
          <a className={NAV_LINK} href="#como-funciona">
            Cómo funciona
          </a>
          <a className={NAV_LINK} href="#preguntas">
            Preguntas
          </a>
        </nav>
      </div>

      <section className="relative isolate grid items-center gap-14 pt-14 pb-18 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)] lg:gap-8 lg:pt-20 lg:pb-24">
        <div aria-hidden="true" className="hud-grid pointer-events-none absolute inset-0 -z-10 opacity-45" />

        <div className="motion-safe:*:animate-rise motion-safe:*:nth-2:[animation-delay:60ms] motion-safe:*:nth-3:[animation-delay:120ms] motion-safe:*:nth-4:[animation-delay:180ms] motion-safe:*:nth-5:[animation-delay:240ms]">
          <Eyebrow>CS2 · demo → vídeo</Eyebrow>
          <h1 className="mb-6 text-[clamp(2.6rem,6.6vw,4.5rem)] leading-none font-bold tracking-[-0.035em] text-balance">
            Tus mejores rondas,{" "}
            <span className="text-primary [text-shadow:0_0_2.5rem_color-mix(in_oklch,var(--primary)_35%,transparent)]">
              montadas a mano
            </span>
            .
          </h1>
          <p className="mb-8 max-w-[44ch] text-[1.1875rem] leading-[1.6] text-pretty text-fg-2">
            Mándame la demo de tu partida de Counter-Strike 2 y te devuelvo un
            vídeo editado con tus jugadas, grabado dentro del juego. Gratis, y
            de una en una.
          </p>
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
            <SendDemoButton />
            <a
              className="py-2.5 font-medium text-fg-2 underline decoration-border-strong underline-offset-[0.2em] transition-colors duration-(--dur-fast) hover:text-fg-1"
              href="#como-funciona"
            >
              Ver cómo funciona
            </a>
          </div>
          <p className="mt-5 text-body-sm text-fg-3">Sin tarjeta y sin publicidad.</p>
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

          {/* The pipeline in the product's own HUD language. Decorative. */}
          <div
            aria-hidden="true"
            className="absolute inset-x-3 bottom-3 rounded-lg border border-border bg-surface-2 px-3.5 py-3 font-mono text-[0.8125rem] leading-[1.6] shadow-lg lg:right-auto lg:bottom-9 lg:-left-22 lg:w-[16.5rem] lg:px-4.5 lg:py-4 lg:shadow-[var(--elev-bevel),0_1.5rem_3rem_-1rem_oklch(0.03_0.02_264/0.9)]"
          >
            <div className="mb-2 flex items-center gap-2 border-b border-border-subtle pb-2.5 text-meta text-fg-3 uppercase">
              <RecDot className="motion-safe:animate-rec-pulse" />
              en proceso
            </div>
            <div className={`${READOUT_ROW} motion-safe:animate-readout-1`}>
              <span>anubis.dem</span>
              <span className="text-fg-3">recibida</span>
            </div>
            <div className={`${READOUT_ROW} motion-safe:animate-readout-2`}>
              <span>grabando en el juego</span>
              <span className="text-fg-3 tabular-nums">1 / 1</span>
            </div>
            <div className="mt-0.5 mb-1 h-0.5 overflow-hidden rounded-full bg-surface-4">
              <span className="block h-full origin-left bg-primary motion-safe:animate-readout-bar" />
            </div>
            <div className={`${READOUT_ROW} motion-safe:animate-readout-3`}>
              <span>tu-video.mp4</span>
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
              La demo guarda la partida entera. La reproduzco en CS2, grabo tus
              rondas desde tu punto de vista y las monto en un vídeo. Nada de
              capturas de pantalla ni de plantillas.
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
          <SectionTitle>Tres pasos y una cola.</SectionTitle>
          {/* A rail with one node per step: vertical on phones, horizontal above. */}
          <ol className="mt-10 grid md:mt-12 md:grid-cols-3">
            {steps.map((step, index) => (
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

      <section id="condiciones" className="mt-22 scroll-mt-8 lg:mt-28">
        <Reveal className={SPLIT}>
          <div>
            <Kicker>Antes de que envíes nada</Kicker>
            <SectionTitle>Esto lo lleva una sola persona.</SectionTitle>
            <p className="mt-6 max-w-[34ch] text-[1.1875rem] leading-[1.55] text-pretty text-fg-1">
              Lo llevo yo solo, con un único equipo que graba las partidas en
              el propio juego. Si tienes prisa, este no es tu sitio. Si quieres
              un vídeo hecho con cuidado, adelante.
            </p>
          </div>
          <ul className="border-t border-border-subtle">
            {rules.map((rule) => (
              <li
                key={rule.label}
                className="grid grid-cols-[5.5rem_minmax(0,1fr)] items-baseline gap-4 border-b border-border-subtle py-4 text-base leading-[1.55]"
              >
                <span className="font-mono text-meta text-fg-3 uppercase">{rule.label}</span>
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
          <SectionTitle className="mx-auto max-w-[18ch]">
            ¿Tienes una partida que merece vídeo?
          </SectionTitle>
          <p className="mx-auto mb-8 max-w-[40ch] text-body-lg leading-[1.6] text-fg-2">
            Sube la demo y deja una nota. Del resto me encargo yo.
          </p>
          <SendDemoButton />
        </section>
      </Reveal>

      <SiteFooter />
    </PageShell>
  );
}
