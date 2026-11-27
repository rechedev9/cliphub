import FilmBackdrop from "@/components/film";
import { PhoneFan, Ticker, Timeline } from "@/components/hud";
import SiteHeader from "@/components/site-header";
import Wordmark from "@/components/wordmark";
import {
  FAQ,
  FULL_DEMO,
  HERO,
  NAV,
  PATHS,
  PHONES,
  PLANS,
  POV_FILM,
  REQUIREMENTS,
  SERVICE,
  SHORT_FEATURES,
  STATS,
  STEPS,
  STREAMS,
  TICKER,
  type Feature,
} from "@/lib/content";
import { DOWNLOAD_URL, INSTALLER_SIZE, RELEASE_VERSION, REPO_URL, SERVICE_CONTACT_URL } from "@/lib/site";

const wrap = "mx-auto w-full max-w-7xl px-5 sm:px-10";
const eyebrow =
  "inline-flex items-center gap-3 font-mono text-xs font-medium uppercase tracking-[0.24em] text-brand sm:text-sm";
const giant = "font-display font-black uppercase italic leading-[0.86] tracking-[-0.01em]";
const h2 = `${giant} hud-glitch mt-5 text-5xl sm:text-7xl lg:text-8xl`;
const lead = "mt-6 max-w-xl text-base leading-relaxed text-muted sm:text-lg";

interface CtaProps {
  href: string;
  label: string;
  solid?: boolean;
}

function Cta({ href, label, solid = false }: CtaProps) {
  const look = solid
    ? "bg-brand text-bg hover:bg-fg"
    : "border-2 border-fg/40 text-fg hover:border-brand hover:text-brand";
  return (
    <a
      href={href}
      className={`hud-cut inline-flex min-h-13 items-center justify-center px-8 font-display text-base font-extrabold uppercase tracking-[0.18em] transition-colors duration-200 ${look}`}
    >
      {label}
    </a>
  );
}

function Eyebrow({ index, label }: { index: string; label: string }) {
  return (
    <p className={eyebrow}>
      <span className="bg-brand px-1.5 text-bg">{index}</span>
      {label}
    </p>
  );
}

interface PathCardProps {
  href: string;
  tag: string;
  title: string;
  text: string;
  cta: string;
  side: "ct" | "t";
}

function PathCard({ href, tag, title, text, cta, side }: PathCardProps) {
  const tone = side === "ct" ? "text-side-ct" : "text-side-t";
  return (
    <a
      href={href}
      className="hud-cut group relative flex flex-col bg-surface/90 p-5 backdrop-blur transition-colors hover:bg-line sm:p-6"
    >
      <span aria-hidden="true" className={`absolute inset-y-0 left-0 w-1.5 ${side === "ct" ? "bg-side-ct" : "bg-side-t"}`} />
      <span className={`font-mono text-[11px] uppercase tracking-[0.22em] ${tone}`}>{tag}</span>
      <span className={`${giant} mt-2 text-4xl`}>{title}</span>
      <span className="mt-2 text-sm leading-relaxed text-muted">{text}</span>
      <span className="mt-5 inline-flex items-center gap-2 font-display text-sm font-extrabold uppercase tracking-[0.18em] text-brand">
        {cta}
        <span aria-hidden="true" className="transition-transform group-hover:translate-x-1">
          →
        </span>
      </span>
    </a>
  );
}

function FeatureCards({ items }: { items: readonly Feature[] }) {
  return (
    <ol className="grid gap-px border border-line bg-line sm:grid-cols-2 lg:grid-cols-4">
      {items.map((f, i) => (
        <li key={f.title} className="group relative bg-bg p-6 transition-colors hover:bg-surface sm:p-7">
          <span className="font-mono text-xs text-faint transition-colors group-hover:text-brand">
            {String(i + 1).padStart(2, "0")}
          </span>
          <h3 className={`${giant} mt-4 text-3xl`}>{f.title}</h3>
          <p className="mt-3 text-sm leading-relaxed text-muted">{f.text}</p>
          <span aria-hidden="true" className="absolute inset-x-0 bottom-0 h-0.5 origin-left scale-x-0 bg-brand transition-transform duration-300 group-hover:scale-x-100" />
        </li>
      ))}
    </ol>
  );
}

const STRUCTURED_DATA = [
  {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "ClipHub Studio",
    applicationCategory: "MultimediaApplication",
    operatingSystem: "Windows",
    description: HERO.lead,
    downloadUrl: DOWNLOAD_URL,
    offers: { "@type": "Offer", price: "0", priceCurrency: "EUR" },
  },
  {
    "@context": "https://schema.org",
    "@type": "FAQPage",
    mainEntity: FAQ.map((item) => ({
      "@type": "Question",
      name: item.q,
      acceptedAnswer: { "@type": "Answer", text: item.a },
    })),
  },
];

export default function Home() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(STRUCTURED_DATA) }} />
      <a
        href="#contenido"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:bg-bg focus:px-3 focus:py-2"
      >
        Saltar al contenido
      </a>

      <SiteHeader nav={NAV} downloadUrl={DOWNLOAD_URL} ctaLabel={HERO.primary} />

      <main id="contenido" tabIndex={-1} className="outline-none">
        <section aria-labelledby="hero-titulo" className="hud-grid relative overflow-hidden pb-16 pt-28 sm:pt-36 lg:pb-24">
          <div aria-hidden="true" className="hud-scanlines pointer-events-none absolute inset-0" />
          <div className={`${wrap} relative grid items-center gap-16 lg:grid-cols-[1.15fr_1fr]`}>
            <div>
              <p className={eyebrow}>
                <span className="flex items-center gap-1.5 border border-rec px-1.5 text-rec">
                  <span className="hud-blink size-1.5 rounded-full bg-rec" />
                  Live
                </span>
                {HERO.kicker}
              </p>
              <div className="relative mt-6">
                <span
                  aria-hidden="true"
                  className="absolute left-[38%] top-1/2 h-3 w-[75%] -translate-x-1/2 -translate-y-1/2 -rotate-[24deg] bg-brand shadow-[0_0_30px_var(--color-brand)] sm:h-5"
                />
                <h1
                  id="hero-titulo"
                  className={`${giant} hud-glitch relative max-w-3xl text-[clamp(3.6rem,13vw,9.5rem)]`}
                >
                  {HERO.title}
                </h1>
              </div>
              <p className={lead}>{HERO.lead}</p>
              <div className="mt-10 grid gap-3 sm:grid-cols-2">
                <PathCard href={DOWNLOAD_URL} side="ct" {...PATHS.tool} />
                <PathCard href={SERVICE_CONTACT_URL} side="t" {...PATHS.service} />
              </div>
              <p className="mt-4 font-mono text-xs uppercase tracking-wider text-faint">
                {RELEASE_VERSION} · {INSTALLER_SIZE} · Windows 10 y 11
              </p>
              <ul className="mt-6 flex flex-wrap gap-x-6 gap-y-2 font-display text-xs font-bold uppercase tracking-[0.2em] text-fg/70">
                {HERO.assurances.map((a) => (
                  <li key={a} className="flex items-center gap-2">
                    <span aria-hidden="true" className="size-1.5 rotate-45 bg-brand" />
                    {a}
                  </li>
                ))}
              </ul>
            </div>
            <div>
              <PhoneFan film={POV_FILM} phones={PHONES} />
              <Timeline />
            </div>
          </div>
        </section>

        <Ticker items={TICKER} />

        <section aria-label="ClipHub Studio en cifras" className="py-14">
          <ul className={`${wrap} grid gap-3 sm:grid-cols-2 lg:grid-cols-4`}>
            {STATS.map((s) => (
              <li key={s.label} className="hud-cut flex items-stretch bg-surface">
                <span className={`w-2 shrink-0 ${s.side === "ct" ? "bg-side-ct" : "bg-side-t"}`} />
                <span className="flex flex-col justify-center px-5 py-5">
                  <span className={`${giant} text-5xl ${s.side === "ct" ? "text-side-ct" : "text-side-t"}`}>{s.value}</span>
                  <span className="mt-2 font-display text-xs font-bold uppercase tracking-[0.16em] text-muted">{s.label}</span>
                </span>
              </li>
            ))}
          </ul>
        </section>

        <section id="como-funciona" aria-labelledby="como-titulo" className="border-t border-line py-24 sm:py-32">
          <div className={wrap}>
            <Eyebrow index="01" label="Cómo funciona" />
            <h2 id="como-titulo" className={`${h2} max-w-5xl`}>
              De la demo al vídeo en tres pasos.
            </h2>
            <ol className="mt-16 grid gap-10 md:grid-cols-3">
              {STEPS.map((step, i) => (
                <li key={step.title} className="relative border-l-2 border-brand pl-6">
                  <span className={`${giant} block text-8xl text-fg/10 sm:text-9xl`}>{i + 1}</span>
                  <h3 className={`${giant} -mt-6 text-3xl`}>{step.title}</h3>
                  <p className="mt-3 max-w-sm text-muted">{step.text}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="shorts" aria-labelledby="shorts-titulo" className="border-t border-line py-24 sm:py-32">
          <div className={wrap}>
            <Eyebrow index="02" label="Shorts 9:16" />
            <h2 id="shorts-titulo" className={`${h2} max-w-5xl`}>
              Hecho para el scroll.
            </h2>
            <p className={lead}>
              Elige las jugadas o deja que Auto-Shorts saque hasta diez de una demo. Cada Short sale con todo lo que necesita
              para que nadie lo pase de largo.
            </p>
            <div className="mt-14">
              <FeatureCards items={SHORT_FEATURES} />
            </div>
          </div>
        </section>

        <section id="demo-completa" aria-labelledby="demo-titulo" className="relative flex min-h-svh items-end overflow-hidden">
          <FilmBackdrop film={POV_FILM} />
          <div
            aria-hidden="true"
            className="absolute inset-0 bg-[linear-gradient(to_top,#000_0%,rgb(0_0_0/0.8)_40%,rgb(0_0_0/0.2)_80%,rgb(0_0_0/0.5)_100%)]"
          />
          <div aria-hidden="true" className="hud-scanlines absolute inset-0" />
          <div className={`${wrap} relative py-24`}>
            <Eyebrow index="03" label="Demo completa · 16:9" />
            <h2 id="demo-titulo" className={`${h2} max-w-5xl`}>
              {FULL_DEMO.title}
            </h2>
            <p className={lead}>{FULL_DEMO.lead}</p>
            <ul className="mt-12 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {FULL_DEMO.points.map((p, i) => (
                <li key={p.title} className="hud-cut bg-bg/70 p-5 backdrop-blur">
                  <span className={`block h-1 w-10 ${i % 2 === 0 ? "bg-side-ct" : "bg-side-t"}`} />
                  <h3 className={`${giant} mt-4 text-2xl`}>{p.title}</h3>
                  <p className="mt-2 text-sm text-muted">{p.text}</p>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section id="streams" aria-labelledby="streams-titulo" className="border-t border-line bg-surface py-24 sm:py-32">
          <div className={`${wrap} grid gap-14 lg:grid-cols-[1fr_1.1fr] lg:items-center`}>
            <div>
              <Eyebrow index="04" label="Clips de stream" />
              <h2 id="streams-titulo" className={h2}>
                {STREAMS.title}
              </h2>
              <p className={lead}>{STREAMS.lead}</p>
              <p className="mt-6 flex flex-wrap gap-2 font-display text-xs font-bold uppercase tracking-[0.2em]">
                {["Twitch", "Kick", "YouTube", ".mp4"].map((s) => (
                  <span key={s} className="border border-line-strong px-3 py-1.5 text-fg/80">
                    {s}
                  </span>
                ))}
              </p>
            </div>
            <ol className="border-t-2 border-brand">
              {STREAMS.points.map((p, i) => (
                <li key={p.title} className="grid grid-cols-[3.5rem_1fr] gap-4 border-b border-line py-6">
                  <span className={`${giant} text-4xl text-brand`}>{String(i + 1).padStart(2, "0")}</span>
                  <span>
                    <span className={`${giant} block text-2xl`}>{p.title}</span>
                    <span className="mt-1 block text-muted">{p.text}</span>
                  </span>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="servicio" aria-labelledby="servicio-titulo" className="relative border-t border-line py-24 sm:py-32">
          <div className={`${wrap} grid gap-14 lg:grid-cols-[1fr_1.1fr] lg:items-start`}>
            <div>
              <Eyebrow index="05" label="Servicio" />
              <h2 id="servicio-titulo" className={h2}>
                {SERVICE.title}
              </h2>
              <p className={lead}>{SERVICE.lead}</p>
              <div className="mt-10">
                <Cta href={SERVICE_CONTACT_URL} label={PATHS.service.cta} solid />
              </div>
              <p className="mt-4 font-mono text-xs uppercase tracking-wider text-faint">{SERVICE.note}</p>
            </div>
            <ol className="grid gap-3">
              {SERVICE.steps.map((step, i) => (
                <li key={step.title} className="hud-cut flex items-stretch bg-surface">
                  <span className="flex w-16 shrink-0 items-center justify-center bg-side-t font-display text-3xl font-black italic text-bg">
                    {i + 1}
                  </span>
                  <span className="px-5 py-5">
                    <span className={`${giant} block text-2xl`}>{step.title}</span>
                    <span className="mt-1 block text-sm text-muted">{step.text}</span>
                  </span>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="planes" aria-labelledby="planes-titulo" className="hud-grid border-t border-line py-24 sm:py-32">
          <div className={wrap}>
            <Eyebrow index="06" label="Elige cómo" />
            <h2 id="planes-titulo" className={`${h2} max-w-5xl`}>
              Tú editas o editamos nosotros.
            </h2>
            <ul className="mt-14 grid gap-4 md:grid-cols-2">
              {PLANS.map((plan) => {
                const ct = plan.id === "studio";
                return (
                  <li key={plan.id} className="hud-cut relative flex flex-col bg-surface p-7 sm:p-9">
                    <span aria-hidden="true" className={`absolute inset-x-0 top-0 h-1.5 ${ct ? "bg-side-ct" : "bg-side-t"}`} />
                    <span className={`font-mono text-xs uppercase tracking-[0.22em] ${ct ? "text-side-ct" : "text-side-t"}`}>
                      {plan.tag}
                    </span>
                    <h3 className={`${giant} mt-3 text-5xl`}>{plan.name}</h3>
                    <p className="mt-2 font-display text-xl font-bold uppercase tracking-[0.08em] text-brand">{plan.price}</p>
                    <ul className="mt-6 flex-1 space-y-3 text-muted">
                      {plan.points.map((point) => (
                        <li key={point} className="flex items-start gap-3">
                          <span aria-hidden="true" className="mt-2 size-1.5 shrink-0 rotate-45 bg-brand" />
                          {point}
                        </li>
                      ))}
                    </ul>
                    <div className="mt-8">
                      <Cta href={ct ? DOWNLOAD_URL : SERVICE_CONTACT_URL} label={plan.cta} solid={ct} />
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        </section>

        <section id="preguntas" aria-labelledby="preguntas-titulo" className="border-t border-line py-24 sm:py-32">
          <div className={`${wrap} grid gap-12 lg:grid-cols-[1fr_1.4fr]`}>
            <div>
              <Eyebrow index="07" label="Preguntas" />
              <h2 id="preguntas-titulo" className={h2}>
                Antes de que preguntes.
              </h2>
            </div>
            <div className="border-t-2 border-brand">
              {FAQ.map((item) => (
                <details key={item.q} className="group border-b border-line">
                  <summary className="flex min-h-12 cursor-pointer list-none items-center justify-between gap-6 py-6 font-display text-lg font-bold uppercase tracking-[0.06em] transition-colors hover:text-brand sm:text-xl [&::-webkit-details-marker]:hidden">
                    {item.q}
                    <span aria-hidden="true" className="text-2xl font-normal text-brand transition-transform group-open:rotate-45">
                      +
                    </span>
                  </summary>
                  <p className="max-w-2xl pb-6 leading-relaxed text-muted">{item.a}</p>
                </details>
              ))}
            </div>
          </div>
        </section>

        <section id="descargar" aria-labelledby="descargar-titulo" className="hud-grid relative overflow-hidden border-t border-line py-24 sm:py-32">
          <div className={`${wrap} relative`}>
            <Eyebrow index="08" label="Empieza ahora" />
            <h2 id="descargar-titulo" className={`${giant} hud-glitch mt-5 text-[clamp(3.6rem,13vw,11rem)]`}>
              Dale a grabar.
            </h2>
            <div className="mt-12 flex flex-col gap-3 sm:flex-row">
              <Cta href={DOWNLOAD_URL} label={HERO.primary} solid />
              <Cta href={SERVICE_CONTACT_URL} label={PATHS.service.cta} />
            </div>
            <p className="mt-5 font-mono text-xs uppercase tracking-wider text-faint">
              {RELEASE_VERSION} · {INSTALLER_SIZE} · Windows 10 y 11
            </p>
            <div className="mt-20 grid gap-12 border-t border-line pt-10 md:grid-cols-2">
              <div>
                <h3 className={eyebrow}>Qué necesitas</h3>
                <ul className="mt-5 space-y-2 text-muted">
                  {REQUIREMENTS.map((r) => (
                    <li key={r} className="flex items-center gap-3">
                      <span aria-hidden="true" className="size-1.5 rotate-45 bg-brand" />
                      {r}
                    </li>
                  ))}
                </ul>
              </div>
              <div>
                <h3 className={eyebrow}>Si Windows avisa al instalar</h3>
                <p className="mt-5 leading-relaxed text-muted">
                  El instalador aún no está firmado, así que Windows puede mostrar «Windows protegió su PC». Pulsa «Más
                  información» y luego «Ejecutar de todas formas». El código y el checksum de cada versión están en{" "}
                  <a href={REPO_URL} className="text-fg underline underline-offset-4 hover:text-brand">
                    GitHub
                  </a>
                  .
                </p>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="border-t border-line pb-28 pt-10 sm:pb-10">
        <div
          className={`${wrap} flex flex-col gap-6 font-display text-xs font-bold uppercase tracking-[0.2em] text-faint sm:flex-row sm:items-center sm:justify-between`}
        >
          <Wordmark />
          <span className="flex flex-wrap gap-8">
            <a href={DOWNLOAD_URL} className="transition-colors hover:text-brand">
              Descargar para Windows
            </a>
            <a href={SERVICE_CONTACT_URL} className="transition-colors hover:text-brand">
              Servicio de edición
            </a>
            <a href={REPO_URL} className="transition-colors hover:text-brand">
              GitHub
            </a>
            <span>© 2026</span>
          </span>
        </div>
      </footer>
    </>
  );
}
