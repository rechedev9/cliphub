import { ArrowLeft, ArrowRight, Download } from "lucide-react";
import CiberStage, { ThreeDLink } from "@/components/ciber-stage";
import LiteVideo from "@/components/lite-video";
import ProductFlow from "@/components/product-flow";
import { TOUR, stationById } from "@/lib/ciber/stations";
import type { StationId } from "@/lib/ciber/stations";
import { DOWNLOAD_URL, INSTALLER_SIZE, RELEASE_VERSION, REPO_URL } from "@/lib/site";

function Wordmark() {
  return (
    <span className="inline-flex items-center gap-2.5">
      <img src="/brand/cliphub-mark.svg" alt="" width={28} height={28} className="size-7" />
      <span className="text-base font-bold tracking-wide">
        ClipHub <span className="font-medium text-muted">Studio</span>
      </span>
    </span>
  );
}

function DownloadButton({ label }: { label: string }) {
  return (
    <a
      href={DOWNLOAD_URL}
      className="inline-flex min-h-12 items-center justify-center gap-2.5 rounded-lg bg-accent px-5 text-base font-semibold text-accent-fg transition-[transform,filter] duration-150 ease-out hover:brightness-110 motion-safe:active:scale-[0.97]"
    >
      <Download className="size-5" aria-hidden="true" strokeWidth={2.2} />
      {label}
    </a>
  );
}

function DownloadMeta() {
  return (
    <p className="mt-2.5 text-sm text-muted">
      Gratis · {RELEASE_VERSION} · {INSTALLER_SIZE} · Windows 10 y 11
    </p>
  );
}

function Still({ name, alt, eager = false }: { name: string; alt: string; eager?: boolean }) {
  return (
    <figure className="lite-only overflow-hidden rounded-xl border border-line bg-surface">
      <img
        src={`/scene/stills/${name}.webp`}
        alt={alt}
        width={1440}
        height={900}
        loading={eager ? "eager" : "lazy"}
        decoding="async"
        className="aspect-[16/10] w-full object-cover"
      />
    </figure>
  );
}

const navLink =
  "inline-flex min-h-9 items-center gap-1.5 rounded-md px-2 text-sm transition-colors pointer-coarse:min-h-11";

/** Every part of Studio at the top of a station panel, the current one marked, so the visitor sees where they are. */
function StationTabs({ id }: { id: StationId }) {
  return (
    <nav aria-label="Partes de Studio" className="only-3d -mx-2 mb-4 flex flex-wrap items-center gap-x-1 gap-y-0.5 border-b border-line pb-3">
      <a href="#inicio" aria-keyshortcuts="Escape" className={`${navLink} text-muted hover:text-fg`}>
        <ArrowLeft className="size-4" aria-hidden="true" />
        Mesa
      </a>
      <span aria-hidden="true" className="mx-1 h-4 w-px bg-line" />
      {TOUR.map((tab) => (
        <a
          key={tab}
          href={`#${tab}`}
          aria-current={tab === id ? "page" : undefined}
          className={`${navLink} text-muted hover:text-fg aria-[current=page]:font-semibold aria-[current=page]:text-fg aria-[current=page]:underline aria-[current=page]:decoration-accent aria-[current=page]:decoration-2 aria-[current=page]:underline-offset-[6px]`}
        >
          {stationById(tab).label}
        </a>
      ))}
    </nav>
  );
}

function PanelNav({ id }: { id: StationId }) {
  const index = TOUR.indexOf(id);
  const prev = TOUR[index - 1];
  const next = TOUR[index + 1];
  return (
    <nav
      aria-label="Anterior y siguiente"
      className="only-3d mt-6 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-4"
    >
      {prev ? (
        <a href={`#${prev}`} aria-keyshortcuts="ArrowLeft" className={`${navLink} min-h-11 text-muted hover:text-fg`}>
          <ArrowLeft className="size-4" aria-hidden="true" />
          {stationById(prev).label}
        </a>
      ) : (
        <span />
      )}
      {next ? (
        <a
          href={`#${next}`}
          aria-keyshortcuts="ArrowRight"
          className={`${navLink} min-h-11 font-medium text-fg hover:text-accent`}
        >
          {stationById(next).label}
          <ArrowRight className="size-4" aria-hidden="true" />
        </a>
      ) : null}
    </nav>
  );
}

function Points({ items }: { items: readonly string[] }) {
  return (
    <ul className="mt-5 space-y-2.5 text-[15px] leading-relaxed text-muted">
      {items.map((item) => (
        <li key={item} className="flex gap-3">
          <span aria-hidden="true" className="mt-[0.6em] size-1.5 shrink-0 rounded-full bg-accent" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}

const panel =
  "lite:mx-auto lite:grid lite:max-w-6xl lite:items-center lite:gap-8 lite:px-5 lite:py-14 lite:sm:px-8 lite:md:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] lite:md:gap-14 lite:md:py-20";
const body = "three-d:p-5 three-d:sm:p-6";
const h2 = "text-2xl font-bold leading-tight text-balance outline-none sm:text-[1.75rem]";
const lead = "mt-3 text-base leading-relaxed text-pretty text-fg/85";

export default function Home() {
  return (
    <>
      <header className="z-30 flex items-center justify-between gap-4 px-4 py-3 sm:px-6 three-d:pointer-events-none three-d:fixed three-d:inset-x-0 three-d:top-0 lite:sticky lite:top-0 lite:border-b lite:border-line lite:bg-bg">
        <a
          href="#inicio"
          className="pointer-events-auto rounded-lg three-d:tape three-d:px-3 three-d:py-2"
        >
          <Wordmark />
        </a>
        <nav aria-label="Secciones" className="lite-only hidden items-center gap-6 text-sm text-muted md:flex">
          <a className="transition-colors hover:text-fg" href="#short">
            Short
          </a>
          <a className="transition-colors hover:text-fg" href="#demo-completa">
            Demo completa
          </a>
          <a className="transition-colors hover:text-fg" href="#clips">
            Clips de stream
          </a>
          <a className="transition-colors hover:text-fg" href="#analisis">
            Análisis
          </a>
        </nav>
        <a
          href={DOWNLOAD_URL}
          className="header-cta pointer-events-auto inline-flex min-h-11 items-center gap-2 rounded-lg border border-line bg-surface px-4 text-sm font-semibold transition-[border-color,transform] duration-150 ease-out hover:border-accent motion-safe:active:scale-[0.97] three-d:tape three-d:rounded-sm three-d:border-0"
        >
          <Download className="size-4" aria-hidden="true" />
          Descargar
        </a>
      </header>

      <CiberStage />

      <main className="ciber-main">
        <section id="inicio" data-panel="inicio" aria-labelledby="inicio-titulo" className={panel}>
          <div className={body}>
            <h1
              id="inicio-titulo"
              tabIndex={-1}
              className="text-[2rem] font-bold leading-[1.08] text-balance outline-none sm:text-[2.6rem] three-d:text-[1.9rem] three-d:sm:text-[2.1rem]"
            >
              De la demo al vídeo, sin editar a mano
            </h1>
            <p className={`${lead} inicio-lead`}>
              Studio lee tu demo de CS2, graba las mejores jugadas y monta el vídeo, con el análisis de la partida al
              lado. Tú eliges el jugador, la música y los efectos.
            </p>
            <ProductFlow />
            <div className="inicio-cta mt-6">
              <DownloadButton label="Descargar para Windows" />
              <DownloadMeta />
            </div>
            <p className="only-3d mt-5 border-t border-line pt-4 text-sm leading-relaxed text-muted">
              Pulsa los objetos de la mesa para ver cada parte de Studio.
              <span className="pointer-coarse:hidden"> También puedes recorrerla con las flechas del teclado.</span>
            </p>
            <p className="lite-only mt-5">
              <ThreeDLink />
            </p>
          </div>
          <Still
            name="mesa"
            eager
            alt="Un puesto de ciber de noche: monitor CRT con una jugada de CS:GO, torre con un CD, teclado, libreta y ticket"
          />
        </section>

        <section id="short" data-panel="short" aria-labelledby="short-titulo" className={panel}>
          <div className={body}>
            <StationTabs id="short" />
            <h2 id="short-titulo" tabIndex={-1} className={h2}>
              Short: tus mejores jugadas en vertical
            </h2>
            <p className={lead}>
              Elige al jugador y ClipHub encuentra sus jugadas, las graba en CS2 con HLAE y las monta en un vídeo 9:16
              listo para subir.
            </p>
            <Points
              items={[
                "«Auto: mejores 60 s» elige el mejor minuto por ti. Las rondas de cinco kills llevan la etiqueta ACE.",
                "Los cortes caen al ritmo de la música, o lo creas sin música.",
                "Killfeed del juego, contador de kills, título automático y portada.",
              ]}
            />
            <PanelNav id="short" />
          </div>
          <div className="lite-only flex justify-center">
            <LiteVideo
              src="/video/killfeed-loop"
              poster="/scene/stills/short-frame.webp"
              label="Un Short de ClipHub: jugada de CS2 en vertical con el killfeed del juego"
            />
          </div>
        </section>

        <section id="demo-completa" data-panel="demo-completa" aria-labelledby="demo-completa-titulo" className={panel}>
          <div className={body}>
            <StationTabs id="demo-completa" />
            <h2 id="demo-completa-titulo" tabIndex={-1} className={h2}>
              Demo completa: la partida entera desde un jugador
            </h2>
            <p className={lead}>
              Graba todas las rondas en 16:9 desde la vista del jugador, con el audio de la partida nivelado y sin
              música de fondo.
            </p>
            <Points
              items={[
                "Once HUD de retransmisión o el HUD original de CS2, y el POV 1:1 con TrueView.",
                "Transiciones entre rondas y rótulos según el origen de la demo: FACEIT, Premier, profesional o local.",
                "Tu intro, tu outro y un vídeo de sponsor después de la ronda 2.",
              ]}
            />
            <PanelNav id="demo-completa" />
          </div>
          <Still name="cd" alt="Primer plano de la bandeja del lector con un CD grabado que pone FINAL SÁB" />
        </section>

        <section id="clips" data-panel="clips" aria-labelledby="clips-titulo" className={panel}>
          <div className={body}>
            <StationTabs id="clips" />
            <h2 id="clips-titulo" tabIndex={-1} className={h2}>
              Clips de stream: tus directos, en Shorts
            </h2>
            <p className={lead}>
              Pega un enlace de Twitch, YouTube o Kick, o sube un MP4. Marca los momentos y cada uno sale como su propio
              Short. No necesita CS2.
            </p>
            <Points
              items={[
                "Cámara + juego, cámara doble o solo juego, con tu facecam encuadrada.",
                "Banner con tu nombre de streamer, música y efecto de color.",
                "Tu edición se guarda en el PC mientras trabajas.",
              ]}
            />
            <PanelNav id="clips" />
          </div>
          <Still
            name="stream"
            alt="El jugador del puesto de al lado, de espaldas, con una webcam encendida sobre su monitor"
          />
        </section>

        <section id="analisis" data-panel="analisis" aria-labelledby="analisis-titulo" className={panel}>
          <div className={body}>
            <StationTabs id="analisis" />
            <h2 id="analisis-titulo" tabIndex={-1} className={h2}>
              Análisis: tus partidas, ronda a ronda
            </h2>
            <p className={lead}>
              Cada demo se analiza en local una sola vez y queda en tu biblioteca con sus equipos, jugadores y rondas.
            </p>
            <Points
              items={[
                "Rondas clasificadas por compra y por patrón, y repetición 2D en el radar.",
                "Fichas de equipos y jugadores con ADR, KAST, WPA y su rol, cada cifra enlazada a sus rondas.",
                "Informes de scouting del rival para el coach, en HTML o PDF.",
                "Integridad puntúa a los diez jugadores para decidir qué revisar a mano. Es un detector de anomalías, no un veredicto.",
                "Desde una ronda puedes grabar el POV de un jugador o mandarla a un Short.",
              ]}
            />
            <PanelNav id="analisis" />
          </div>
          <Still
            name="notepad"
            alt="Una libreta con notas a mano de la final: rondas, ADR, KAST y la ronda 19 marcada para hacer un Short"
          />
        </section>

        <section id="descargar" data-panel="descargar" aria-labelledby="descargar-titulo" className={panel}>
          <div className={body}>
            <StationTabs id="descargar" />
            <h2 id="descargar-titulo" tabIndex={-1} className={h2}>
              Descarga ClipHub Studio
            </h2>
            <p className={lead}>
              Las demos se analizan, graban y editan en tu PC: no necesitas cuenta y tus archivos no se suben a ningún
              sitio.
            </p>
            <div className="mt-6">
              <DownloadButton label="Descargar para Windows" />
              <DownloadMeta />
            </div>
            <h3 className="mt-7 text-sm font-semibold">Qué necesitas</h3>
            <Points
              items={[
                "Windows 10 u 11 de 64 bits.",
                "Counter-Strike 2 instalado con Steam.",
                "HLAE oficial: Studio lo instala y lo actualiza solo.",
                "Una tarjeta gráfica dedicada para grabar con la mejor calidad.",
                "Alrededor de 1 GB libre en el disco.",
              ]}
            />
            <h3 className="mt-7 text-sm font-semibold">Si Windows avisa al instalar</h3>
            <p className="mt-2 text-[15px] leading-relaxed text-muted">
              El instalador aún no está firmado, así que Windows puede mostrar «Windows protegió su PC». Pulsa «Más
              información» y luego «Ejecutar de todas formas». El código y el checksum de cada versión están en{" "}
              <a
                href={REPO_URL}
                className="font-medium text-fg underline decoration-line underline-offset-4 hover:decoration-accent"
              >
                GitHub
              </a>
              .
            </p>
            <p className="only-3d mt-6 border-t border-line pt-4 text-xs text-muted">© 2026 ClipHub</p>
            <PanelNav id="descargar" />
          </div>
          <Still name="ticket" alt="El ticket del ciber: puesto 07, una hora, 1,50 euros" />
        </section>
      </main>

      <footer className="site-footer border-t border-line px-5 py-10 sm:px-8">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 text-sm text-muted sm:flex-row sm:items-center sm:justify-between">
          <p>Gratis y en local. ClipHub nunca sube tus demos, clips ni vídeos.</p>
          <p className="flex items-center gap-5">
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
