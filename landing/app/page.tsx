import { ChevronDown } from "lucide-react";
import FilmBackdrop from "@/components/film";
import ProductViewer from "@/components/product-viewer";
import SiteHeader from "@/components/site-header";
import Wordmark from "@/components/wordmark";
import {
  CAPABILITIES,
  COMMS,
  COMPARISON,
  CREATOR_POINTS,
  FAQ,
  FILMS,
  HERO,
  NAV,
  REQUIREMENTS,
  STATS,
  STEPS,
  TEAM_POINTS,
  VIEWER_TABS,
} from "@/lib/content";
import { ANALYTICS_URL, DOWNLOAD_URL, INSTALLER_SIZE, RELEASE_VERSION, REPO_URL } from "@/lib/site";

const wrap = "mx-auto w-full max-w-7xl px-5 sm:px-10";
const eyebrow = "font-display text-xs font-bold uppercase tracking-[0.3em] text-muted sm:text-sm";
const giant = "font-display font-extrabold uppercase leading-[0.88] tracking-[-0.01em]";
const h2 = `${giant} mt-4 text-5xl sm:text-7xl lg:text-8xl`;
const lead = "mt-6 max-w-xl text-base leading-relaxed text-muted sm:text-lg";

interface CtaProps {
  href: string;
  label: string;
  solid?: boolean;
}

function Cta({ href, label, solid = false }: CtaProps) {
  const look = solid
    ? "border-fg bg-fg text-bg hover:bg-transparent hover:text-fg"
    : "border-fg text-fg hover:bg-fg hover:text-bg";
  return (
    <a
      href={href}
      className={`inline-flex min-h-12 items-center justify-center border-2 px-7 font-display text-sm font-bold uppercase tracking-[0.2em] transition-colors duration-300 ${look}`}
    >
      {label}
    </a>
  );
}

function Scrim() {
  return (
    <div
      aria-hidden="true"
      className="absolute inset-0 bg-[linear-gradient(to_top,#000_0%,rgb(0_0_0/0.75)_35%,rgb(0_0_0/0.15)_75%,rgb(0_0_0/0.45)_100%),linear-gradient(to_right,rgb(0_0_0/0.7)_0%,transparent_65%)]"
    />
  );
}

const STRUCTURED_DATA = [
  {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "ClipHub",
    applicationCategory: "SportsApplication",
    operatingSystem: "Web, Windows",
    description: HERO.lead,
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
        href="#analisis"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:bg-bg focus:px-3 focus:py-2"
      >
        Saltar al contenido
      </a>

      <SiteHeader nav={NAV} appUrl={ANALYTICS_URL} ctaLabel={HERO.primary} />

      <main>
        <section aria-labelledby="hero-titulo" className="relative flex min-h-svh items-end overflow-hidden">
          <FilmBackdrop film={FILMS.hero} priority />
          <Scrim />
          <div className={`${wrap} relative pb-20 pt-32 sm:pb-24`}>
            <p className={eyebrow}>{HERO.kicker}</p>
            <h1 id="hero-titulo" className={`${giant} mt-5 max-w-5xl text-[clamp(3.5rem,11vw,10rem)]`}>
              {HERO.title}
            </h1>
            <p className={lead}>{HERO.lead}</p>
            <div className="mt-10 flex flex-col gap-3 sm:flex-row">
              <Cta href={ANALYTICS_URL} label={HERO.primary} solid />
              <Cta href={DOWNLOAD_URL} label={HERO.secondary} />
            </div>
            <ul className="mt-6 flex flex-wrap gap-x-6 gap-y-2 font-display text-xs font-bold uppercase tracking-[0.2em] text-fg/70">
              {HERO.assurances.map((a) => (
                <li key={a} className="flex items-center gap-2">
                  <span aria-hidden="true" className="size-1.5 bg-brand" />
                  {a}
                </li>
              ))}
            </ul>
          </div>
          <ChevronDown aria-hidden="true" className="absolute bottom-6 left-1/2 size-6 -translate-x-1/2 text-fg/60" />
        </section>

        <section aria-label="ClipHub en cifras" className="border-y border-line">
          <ul className={`${wrap} grid divide-line py-14 sm:grid-cols-3 sm:divide-x`}>
            {STATS.map((s) => (
              <li key={s.label} className="py-4 sm:px-10 sm:first:pl-0">
                <span className={`${giant} block text-7xl sm:text-8xl`}>{s.value}</span>
                <span className={`${eyebrow} mt-3 block`}>{s.label}</span>
              </li>
            ))}
          </ul>
        </section>

        <section id="como-funciona" aria-labelledby="como-titulo" className="py-24 sm:py-32">
          <div className={wrap}>
            <p className={eyebrow}>Cómo funciona</p>
            <h2 id="como-titulo" className={`${h2} max-w-5xl`}>
              De la demo al anti-strat en tres pasos.
            </h2>
            <ol className="mt-16 grid gap-12 md:grid-cols-3">
              {STEPS.map((step, i) => (
                <li key={step.title}>
                  <span className={`${giant} block text-8xl text-fg/15 sm:text-9xl`}>{i + 1}</span>
                  <h3 className={`${giant} mt-2 text-3xl`}>{step.title}</h3>
                  <p className="mt-3 max-w-sm text-muted">{step.text}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="analisis" aria-labelledby="analisis-titulo" className="relative flex min-h-svh items-end overflow-hidden">
          <FilmBackdrop film={FILMS.duels} />
          <Scrim />
          <div className={`${wrap} relative py-24`}>
            <p className={eyebrow}>Preparar al rival</p>
            <h2 id="analisis-titulo" className={`${h2} max-w-4xl`}>
              Cada duelo. Cada ruta. Cada ronda.
            </h2>
            <p className={lead}>
              ClipHub lee la demo entera, no media partida, y te da los números con las rondas detrás. El anti-strat
              sale de lo que pasó, no de una impresión.
            </p>
          </div>
        </section>

        <section aria-labelledby="pantallas-titulo" className="py-24 sm:py-32">
          <div className={wrap}>
            <p className={eyebrow}>El producto</p>
            <h2 id="pantallas-titulo" className={h2}>
              El rival, en números.
            </h2>
            <div className="mt-14">
              <ProductViewer tabs={VIEWER_TABS} />
            </div>
            <div className="mt-14 flex flex-col gap-3 sm:flex-row sm:items-center sm:gap-6">
              <Cta href={ANALYTICS_URL} label="Pruébalo con tus demos" solid />
              <span className="text-sm text-muted">Gratis. Sin tarjeta. Sin subir tus demos.</span>
            </div>
          </div>
        </section>

        <section aria-labelledby="capacidades-titulo" className="border-t border-line py-24 sm:py-32">
          <div className={wrap}>
            <h2 id="capacidades-titulo" className="sr-only">
              Qué puedes hacer
            </h2>
            <ol className="grid gap-x-12 gap-y-14 sm:grid-cols-2 lg:grid-cols-3">
              {CAPABILITIES.map((c, i) => (
                <li key={c.title} className="border-t-2 border-fg pt-6">
                  <span className="font-mono text-sm text-faint">{String(i + 1).padStart(2, "0")}</span>
                  <h3 className={`${giant} mt-3 text-4xl`}>{c.title}</h3>
                  <p className="mt-3 text-muted">{c.text}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section id="comms" aria-labelledby="comms-titulo" className="border-t border-line bg-surface py-24 sm:py-32">
          <div className={`${wrap} grid gap-16 lg:grid-cols-[1fr_1.1fr] lg:items-center`}>
            <div>
              <p className={eyebrow}>Comms de voz</p>
              <h2 id="comms-titulo" className={h2}>
                {COMMS.title}
              </h2>
              <p className={lead}>{COMMS.lead}</p>
            </div>
            <figure>
              <figcaption className="flex items-center justify-between border-b border-line pb-4 font-mono text-xs uppercase tracking-wider text-faint">
                <span>Ronda 14 · Mirage · T</span>
                <span>Ejemplo</span>
              </figcaption>
              <ol>
                {COMMS.sample.map((line) => (
                  <li
                    key={line.clock}
                    className="grid grid-cols-[4.5rem_1fr] gap-4 border-b border-line py-6 sm:grid-cols-[6rem_1fr]"
                  >
                    <span className={`${giant} text-3xl text-side-t sm:text-4xl`}>{line.clock}</span>
                    <span>
                      <span className="font-display text-xs font-bold uppercase tracking-[0.2em] text-faint">
                        {line.speaker} · {line.phase}
                      </span>
                      <span className="mt-1 block text-lg leading-snug sm:text-xl">{line.text}</span>
                    </span>
                  </li>
                ))}
              </ol>
            </figure>
          </div>
        </section>

        <section id="equipo" aria-labelledby="equipo-titulo" className="relative flex min-h-svh items-end overflow-hidden">
          <FilmBackdrop film={FILMS.team} />
          <Scrim />
          <div className={`${wrap} relative py-24`}>
            <Wordmark analytics />
            <h2 id="equipo-titulo" className={`${h2} max-w-5xl`}>
              Todo el staff. Una biblioteca.
            </h2>
            <p className={lead}>
              ClipHub Analytics en el navegador y ClipHub Studio en Windows comparten la misma biblioteca. El coach
              prepara desde el portátil y el analista sigue en el PC del bootcamp.
            </p>
            <ul className="mt-10 flex flex-col gap-3 sm:flex-row sm:gap-10">
              {TEAM_POINTS.map((p) => (
                <li key={p} className="font-display text-sm font-bold uppercase tracking-[0.18em] text-fg/90">
                  {p}
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section id="comparativa" aria-labelledby="comparativa-titulo" className="py-24 sm:py-32">
          <div className={wrap}>
            <p className={eyebrow}>Comparativa</p>
            <h2 id="comparativa-titulo" className={`${h2} max-w-5xl`}>
              Lo que otros cobran, aquí es gratis.
            </h2>
            <table className="mt-14 w-full border-collapse text-left">
              <thead>
                <tr className="border-b-2 border-fg font-display text-xs font-bold uppercase tracking-[0.2em]">
                  <th scope="col" className="py-4 pr-4">
                    <span className="sr-only">Función</span>
                  </th>
                  <th scope="col" className="py-4 pr-4">
                    {COMPARISON.ours}
                  </th>
                  <th scope="col" className="py-4 text-faint">
                    {COMPARISON.theirs}
                  </th>
                </tr>
              </thead>
              <tbody>
                {COMPARISON.rows.map((row) => (
                  <tr key={row.feature} className="border-b border-line">
                    <th
                      scope="row"
                      className="py-5 pr-4 align-top font-display text-sm font-bold uppercase tracking-[0.12em] sm:text-base"
                    >
                      {row.feature}
                    </th>
                    <td className="py-5 pr-4 align-top text-sm sm:text-lg">{row.ours}</td>
                    <td className="py-5 align-top text-sm text-faint sm:text-lg">{row.theirs}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section id="creadores" aria-labelledby="creadores-titulo" className="relative flex min-h-svh items-end overflow-hidden">
          <FilmBackdrop film={FILMS.pro} />
          <Scrim />
          <div className={`${wrap} relative py-24`}>
            <p className={eyebrow}>Para creadores · Solo Windows</p>
            <h2 id="creadores-titulo" className={`${h2} max-w-5xl`}>
              De la demo al vídeo.
            </h2>
            <p className={lead}>
              ClipHub Studio añade las herramientas de clipping: graba las jugadas dentro de CS2 y monta el vídeo por ti.
            </p>
            <ul className="mt-12 grid gap-8 border-t border-fg/30 pt-8 sm:grid-cols-3">
              {CREATOR_POINTS.map((p) => (
                <li key={p.title}>
                  <h3 className={`${giant} text-3xl`}>{p.title}</h3>
                  <p className="mt-2 text-sm text-muted">{p.text}</p>
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section id="preguntas" aria-labelledby="preguntas-titulo" className="border-t border-line py-24 sm:py-32">
          <div className={`${wrap} grid gap-12 lg:grid-cols-[1fr_1.4fr]`}>
            <div>
              <p className={eyebrow}>Preguntas</p>
              <h2 id="preguntas-titulo" className={h2}>
                Antes de que preguntes.
              </h2>
            </div>
            <div className="border-t-2 border-fg">
              {FAQ.map((item) => (
                <details key={item.q} className="group border-b border-line">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-6 py-6 font-display text-lg font-bold uppercase tracking-[0.06em] sm:text-xl [&::-webkit-details-marker]:hidden">
                    {item.q}
                    <span aria-hidden="true" className="text-2xl font-normal transition-transform group-open:rotate-45">
                      +
                    </span>
                  </summary>
                  <p className="max-w-2xl pb-6 leading-relaxed text-muted">{item.a}</p>
                </details>
              ))}
            </div>
          </div>
        </section>

        <section id="descargar" aria-labelledby="descargar-titulo" className="border-t border-line py-24 sm:py-32">
          <div className={wrap}>
            <p className={eyebrow}>Empieza ahora</p>
            <h2 id="descargar-titulo" className={`${giant} mt-4 text-[clamp(3.5rem,12vw,11rem)]`}>
              Gratis. Hoy.
            </h2>
            <div className="mt-12 flex flex-col gap-3 sm:flex-row">
              <Cta href={ANALYTICS_URL} label={HERO.primary} solid />
              <Cta href={DOWNLOAD_URL} label={HERO.secondary} />
            </div>
            <p className="mt-5 font-mono text-xs uppercase tracking-wider text-faint">
              {RELEASE_VERSION} · {INSTALLER_SIZE} · Windows 10 y 11
            </p>
            <div className="mt-20 grid gap-12 border-t border-line pt-10 md:grid-cols-2">
              <div>
                <h3 className={eyebrow}>Qué necesitas para Studio</h3>
                <ul className="mt-5 space-y-2 text-muted">
                  {REQUIREMENTS.map((r) => (
                    <li key={r}>{r}</li>
                  ))}
                </ul>
              </div>
              <div>
                <h3 className={eyebrow}>Si Windows avisa al instalar</h3>
                <p className="mt-5 leading-relaxed text-muted">
                  El instalador aún no está firmado, así que Windows puede mostrar «Windows protegió su PC». Pulsa «Más
                  información» y luego «Ejecutar de todas formas». El código y el checksum de cada versión están en{" "}
                  <a href={REPO_URL} className="text-fg underline underline-offset-4">
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
          <span>ClipHub © 2026</span>
          <span className="flex flex-wrap gap-8">
            <a href={ANALYTICS_URL} className="transition-colors hover:text-fg">
              ClipHub Analytics
            </a>
            <a href={DOWNLOAD_URL} className="transition-colors hover:text-fg">
              Studio para Windows
            </a>
            <a href={REPO_URL} className="transition-colors hover:text-fg">
              GitHub
            </a>
          </span>
        </div>
      </footer>
    </>
  );
}
