import { ArrowRight, Check, Download, Minus } from "lucide-react";
import LiteVideo from "@/components/lite-video";
import ProductViewer from "@/components/product-viewer";
import ShotFigure from "@/components/shot";
import Wordmark from "@/components/wordmark";
import {
  COMMS,
  COMPARISON,
  CREATOR_POINTS,
  HERO,
  NAV,
  PREP_POINTS,
  PREP_SHOTS,
  REQUIREMENTS,
  STATS,
  TEAM_POINTS,
  TEAM_SHOT,
  VIEWER_TABS,
} from "@/lib/content";
import { ANALYTICS_URL, DOWNLOAD_URL, INSTALLER_SIZE, RELEASE_VERSION, REPO_URL } from "@/lib/site";

const wrap = "mx-auto w-full max-w-6xl px-4 sm:px-6";
const eyebrow = "font-mono text-xs uppercase tracking-[0.16em] text-brand";
const h2 = "mt-3 max-w-2xl text-3xl font-semibold leading-tight tracking-tight sm:text-4xl";
const lead = "mt-4 max-w-2xl text-base leading-relaxed text-muted sm:text-lg";

function PrimaryCta({ label }: { label: string }) {
  return (
    <a
      href={ANALYTICS_URL}
      className="inline-flex items-center gap-2 rounded-md bg-brand px-5 py-3 text-sm font-semibold text-brand-ink transition-[filter] hover:brightness-110"
    >
      {label}
      <ArrowRight aria-hidden="true" className="size-4" />
    </a>
  );
}

function DownloadCta({ label }: { label: string }) {
  return (
    <a
      href={DOWNLOAD_URL}
      className="inline-flex items-center gap-2 rounded-md border border-line-strong px-5 py-3 text-sm font-semibold text-fg transition-colors hover:border-faint"
    >
      <Download aria-hidden="true" className="size-4" />
      {label}
    </a>
  );
}

function PointList({ items }: { items: readonly { title: string; text: string }[] }) {
  return (
    <ul className="grid gap-x-10 gap-y-6 sm:grid-cols-2">
      {items.map((item) => (
        <li key={item.title} className="border-t border-line pt-4">
          <h3 className="font-semibold">{item.title}</h3>
          <p className="mt-1.5 text-sm leading-relaxed text-muted">{item.text}</p>
        </li>
      ))}
    </ul>
  );
}

function Checks({ items }: { items: readonly string[] }) {
  return (
    <ul className="space-y-2.5">
      {items.map((item) => (
        <li key={item} className="flex gap-3 text-sm leading-relaxed text-muted">
          <Check aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-brand" />
          {item}
        </li>
      ))}
    </ul>
  );
}

export default function Home() {
  return (
    <>
      <a
        href="#analisis"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:bg-bg focus:px-3 focus:py-2"
      >
        Saltar al contenido
      </a>

      <header className="sticky top-0 z-40 border-b border-line bg-bg/85 backdrop-blur">
        <div className={`${wrap} flex h-14 items-center justify-between gap-4`}>
          <a href="#" aria-label="ClipHub, inicio">
            <Wordmark />
          </a>
          <nav aria-label="Secciones" className="hidden items-center gap-6 text-sm text-muted md:flex">
            {NAV.map((item) => (
              <a key={item.href} href={item.href} className="transition-colors hover:text-fg">
                {item.label}
              </a>
            ))}
          </nav>
          <a
            href={ANALYTICS_URL}
            className="rounded-md bg-brand px-3.5 py-2 text-sm font-semibold text-brand-ink transition-[filter] hover:brightness-110"
          >
            Abrir ClipHub
          </a>
        </div>
      </header>

      <main>
        <section aria-labelledby="hero-titulo" className="relative overflow-hidden">
          <div aria-hidden="true" className="grid-bg pointer-events-none absolute inset-0" />
          <div className={`${wrap} relative pb-16 pt-14 sm:pt-20`}>
            <p className={eyebrow}>{HERO.kicker}</p>
            <h1
              id="hero-titulo"
              className="mt-4 max-w-3xl text-4xl font-semibold leading-[1.05] tracking-tight sm:text-6xl"
            >
              {HERO.title}
            </h1>
            <p className={lead}>{HERO.lead}</p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <PrimaryCta label={HERO.primary} />
              <DownloadCta label={HERO.secondary} />
            </div>
            <ul className="mt-8 flex flex-wrap gap-x-8 gap-y-3 font-mono text-sm">
              {STATS.map((s) => (
                <li key={s.label}>
                  <span className="text-fg">{s.value}</span> <span className="text-faint">{s.label}</span>
                </li>
              ))}
            </ul>
            <div className="mt-12">
              <ProductViewer tabs={VIEWER_TABS} />
            </div>
          </div>
        </section>

        <section id="analisis" aria-labelledby="analisis-titulo" className="border-t border-line py-20">
          <div className={wrap}>
            <p className={eyebrow}>Preparar al rival</p>
            <h2 id="analisis-titulo" className={h2}>
              Llega a la partida sabiendo qué va a hacer el otro equipo.
            </h2>
            <p className={lead}>
              Importa sus demos o abre partidos pro ya analizados. ClipHub lee cada ronda entera y te da los números con
              las rondas detrás, para que el anti-strat salga de lo que pasó y no de una impresión.
            </p>
            <div className="mt-12">
              <PointList items={PREP_POINTS} />
            </div>
            <div className="mt-14 grid gap-6 lg:grid-cols-2">
              {PREP_SHOTS.map((shot) => (
                <ShotFigure key={shot.src} shot={shot} />
              ))}
            </div>
          </div>
        </section>

        <section id="comms" aria-labelledby="comms-titulo" className="border-t border-line bg-surface py-20">
          <div className={`${wrap} grid items-start gap-12 lg:grid-cols-2`}>
            <div>
              <p className={eyebrow}>Comms de voz</p>
              <h2 id="comms-titulo" className={h2}>
                {COMMS.title}
              </h2>
              <p className={lead}>{COMMS.lead}</p>
              <div className="mt-8">
                <Checks items={COMMS.points} />
              </div>
            </div>
            <figure className="rounded-lg border border-line bg-bg">
              <figcaption className="flex items-center justify-between gap-3 border-b border-line px-4 py-3 font-mono text-xs text-faint">
                <span>Ronda 14 · Mirage · lado T</span>
                <span className="rounded border border-line px-1.5 py-0.5 uppercase tracking-wider">Ejemplo</span>
              </figcaption>
              <ol className="divide-y divide-line">
                {COMMS.sample.map((line) => (
                  <li key={line.clock} className="grid grid-cols-[3.25rem_1fr] gap-3 px-4 py-3">
                    <span className="font-mono text-sm text-side-t">{line.clock}</span>
                    <span>
                      <span className="font-mono text-[11px] uppercase tracking-wider text-faint">
                        {line.speaker} · {line.phase}
                      </span>
                      <span className="mt-0.5 block text-sm text-fg">{line.text}</span>
                    </span>
                  </li>
                ))}
              </ol>
            </figure>
          </div>
        </section>

        <section id="equipo" aria-labelledby="equipo-titulo" className="border-t border-line py-20">
          <div className={wrap}>
            <p className={eyebrow}>Trabajo en equipo</p>
            <h2 id="equipo-titulo" className={h2}>
              Un sitio para todo el staff, en el navegador o en Windows.
            </h2>
            <p className={lead}>
              ClipHub Analytics en el navegador y ClipHub Studio en Windows comparten la misma biblioteca. El coach
              prepara desde el portátil y el analista sigue en el PC del bootcamp.
            </p>
            <div className="mt-6">
              <Wordmark analytics />
            </div>
            <div className="mt-12 grid items-start gap-12 lg:grid-cols-[1fr_1.3fr]">
              <PointList items={TEAM_POINTS} />
              <ShotFigure shot={TEAM_SHOT} />
            </div>
          </div>
        </section>

        <section
          id="comparativa"
          aria-labelledby="comparativa-titulo"
          className="border-t border-line bg-surface py-20"
        >
          <div className={wrap}>
            <p className={eyebrow}>Comparativa</p>
            <h2 id="comparativa-titulo" className={h2}>
              Lo que otras herramientas cobran, aquí viene incluido.
            </h2>
            <div className="mt-10 overflow-hidden rounded-lg border border-line">
              <table className="w-full border-collapse text-left text-xs sm:text-sm">
                <thead>
                  <tr className="border-b border-line bg-bg font-mono text-xs uppercase tracking-wider text-faint">
                    <th scope="col" className="px-3 py-3 sm:px-4 font-medium">
                      <span className="sr-only">Función</span>
                    </th>
                    <th scope="col" className="px-3 py-3 sm:px-4 font-medium text-brand">
                      {COMPARISON.ours}
                    </th>
                    <th scope="col" className="px-3 py-3 sm:px-4 font-medium">
                      {COMPARISON.theirs}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {COMPARISON.rows.map((row) => (
                    <tr key={row.feature} className="border-b border-line last:border-0">
                      <th scope="row" className="px-3 py-3 sm:px-4 font-medium text-fg">
                        {row.feature}
                      </th>
                      <td className="px-3 py-3 sm:px-4">
                        <span className="inline-flex items-center gap-2 text-fg">
                          <Check aria-hidden="true" className="hidden size-4 shrink-0 text-brand sm:block" />
                          {row.ours}
                        </span>
                      </td>
                      <td className="px-3 py-3 sm:px-4">
                        <span className="inline-flex items-center gap-2 text-faint">
                          <Minus aria-hidden="true" className="hidden size-4 shrink-0 sm:block" />
                          {row.theirs}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </section>

        <section id="creadores" aria-labelledby="creadores-titulo" className="border-t border-line py-20">
          <div className={`${wrap} grid items-center gap-12 lg:grid-cols-[1.2fr_1fr]`}>
            <div>
              <p className={eyebrow}>Para creadores · solo Windows</p>
              <h2 id="creadores-titulo" className={h2}>
                Y cuando la jugada merece verse, la conviertes en vídeo.
              </h2>
              <p className={lead}>
                ClipHub Studio añade a todo lo anterior las herramientas de clipping. Graba las jugadas dentro de CS2 y
                monta el vídeo por ti.
              </p>
              <div className="mt-10">
                <PointList items={CREATOR_POINTS} />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <LiteVideo
                src="/video/killfeed-loop"
                poster="/video/killfeed-loop.webp"
                label="Un Short de ClipHub: jugada de CS2 en vertical con el killfeed del juego"
              />
              <LiteVideo
                src="/video/hero-loop"
                poster="/video/hero-loop.webp"
                label="Un clip de stream de ClipHub: la partida abajo y la facecam del streamer encuadrada arriba"
              />
            </div>
          </div>
        </section>

        <section id="descargar" aria-labelledby="descargar-titulo" className="border-t border-line bg-surface py-20">
          <div className={`${wrap} grid gap-12 lg:grid-cols-2`}>
            <div>
              <p className={eyebrow}>Empieza ahora</p>
              <h2 id="descargar-titulo" className={h2}>
                Ábrelo en el navegador o descárgalo para Windows.
              </h2>
              <p className={lead}>El análisis es el mismo en los dos. Studio añade la grabación y el montaje de vídeo.</p>
              <div className="mt-8 flex flex-wrap gap-3">
                <PrimaryCta label={HERO.primary} />
                <DownloadCta label={HERO.secondary} />
              </div>
              <p className="mt-4 font-mono text-xs text-faint">
                Gratis · {RELEASE_VERSION} · {INSTALLER_SIZE} · Windows 10 y 11
              </p>
            </div>
            <div className="space-y-8">
              <div>
                <h3 className="font-semibold">Qué necesitas para Studio</h3>
                <div className="mt-3">
                  <Checks items={REQUIREMENTS} />
                </div>
              </div>
              <div>
                <h3 className="font-semibold">Si Windows avisa al instalar</h3>
                <p className="mt-2 text-sm leading-relaxed text-muted">
                  El instalador aún no está firmado, así que Windows puede mostrar «Windows protegió su PC». Pulsa «Más
                  información» y luego «Ejecutar de todas formas». El código y el checksum de cada versión están en{" "}
                  <a
                    href={REPO_URL}
                    className="text-fg underline decoration-line-strong underline-offset-4 hover:decoration-brand"
                  >
                    GitHub
                  </a>
                  .
                </p>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="border-t border-line py-10">
        <div className={`${wrap} flex flex-col gap-4 text-sm text-faint sm:flex-row sm:items-center sm:justify-between`}>
          <Wordmark />
          <p className="flex flex-wrap items-center gap-5">
            <a href={ANALYTICS_URL} className="transition-colors hover:text-fg">
              ClipHub Analytics
            </a>
            <a href={REPO_URL} className="transition-colors hover:text-fg">
              Código en GitHub
            </a>
            <span>© 2026 ClipHub</span>
          </p>
        </div>
      </footer>
    </>
  );
}
