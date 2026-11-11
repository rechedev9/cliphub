import { Download } from "lucide-react";
import PhoneFrame from "@/components/phone-frame";
import ProductFlow from "@/components/product-flow";
import Reveal from "@/components/reveal";
import SectionVideo from "@/components/section-video";
import Shot from "@/components/shot";
import { DOWNLOAD_URL, INSTALLER_SIZE, RELEASE_VERSION, REPO_URL } from "@/lib/site";

const NAV = [
  { href: "#short", label: "Short" },
  { href: "#demo-completa", label: "Demo completa" },
  { href: "#streams", label: "Streams" },
  { href: "#analisis", label: "Análisis" },
  { href: "#asistentes", label: "Asistentes" },
] as const;

const FACTS = ["Gratis", "En tu PC, sin subir nada", "Graba en CS2 con HLAE", "Windows 10 y 11"] as const;

const STEPS = [
  {
    title: "Importa tu demo",
    body: "Arrastra el .dem, un .zip o un .rar, o trae la partida desde Steam o FACEIT.",
  },
  {
    title: "Studio la graba en CS2 con HLAE",
    body: "Elige jugador, rondas y HUD. Studio pilota CS2 y graba cada plano por ti.",
  },
  {
    title: "Te da el MP4 editado",
    body: "Con cortes al ritmo, textos, portada y audio nivelado, listo para subir.",
  },
] as const;

const SOURCES = [
  "Archivos .dem, .zip o .rar (hasta 10 a la vez).",
  "Código de compartir de Steam y tus partidas recientes.",
  "Jugadores de FACEIT que sigues.",
  "Partidas profesionales ya analizadas.",
  "Series BO3 y BO5 con su veto.",
  "Streams de Twitch, YouTube, Kick o un MP4.",
] as const;

const SHORT_POINTS = [
  "«Elegir las mejores» arma el mejor minuto y marca los ACE.",
  "6 efectos de kill, 6 transiciones y 15 temas con cortes al ritmo.",
  "Texto de apertura y cierre, portada y banner de afiliado.",
  "Modo Muertes con los segundos antes de cada muerte.",
  "Multi-POV con 2 a 5 jugadores del equipo.",
  "Una serie entera con un vídeo por mapa.",
  "Formato 9:16 o 16:9.",
] as const;

const DEMO_POINTS = [
  "16:9 y audio nivelado.",
  "11 HUD de retransmisión o el HUD de CS2.",
  "POV 1:1 con TrueView.",
  "Voces del equipo.",
  "Transiciones entre rondas.",
  "Tu intro, outro y sponsor después de la ronda 2.",
] as const;

const STREAM_POINTS = [
  "Pega un enlace o sube un MP4.",
  "Studio encuentra tus kills y los picos de voz.",
  "Cámara + juego, cámara doble o solo juego.",
  "Banner, música y color.",
  "Un Short por momento o un resumen 16:9.",
  "No necesita CS2.",
] as const;

const VIDEO_POINTS = [
  "Estados en cola, grabando, en edición y listo.",
  "Reintentar un vídeo fallido o cambiar la música sin volver a grabar.",
  "El asistente de YouTube propone título, alternativas, descripción, etiquetas y la mejor hora.",
  "Tú subes el vídeo, ClipHub nunca publica por ti.",
] as const;

const ANALYSIS = [
  {
    shot: "replay",
    title: "Replay 2D",
    body: "La ronda sobre el mapa, con los jugadores y la cámara donde quieras.",
    alt: "Replay 2D de una ronda sobre el mapa, con los jugadores y sus movimientos",
  },
  {
    shot: "heatmap",
    title: "Mapa de calor",
    body: "Dónde se juega cada ronda, por equipo y por zona.",
    alt: "Mapa de calor del equipo sobre Mirage con las zonas más jugadas",
  },
  {
    shot: "partida",
    title: "Resumen de la partida",
    body: "Rondas, compras y economía en una línea de tiempo.",
    alt: "Resumen de la partida con la línea de tiempo de las rondas y el marcador",
  },
  {
    shot: "equipo",
    title: "Equipos",
    body: "Lados, plantilla y estadísticas de cada equipo.",
    alt: "Página de equipo con sus lados, su plantilla y las estadísticas de cada jugador",
  },
  {
    shot: "preparar",
    title: "Preparar al rival",
    body: "El plan de la próxima partida, con sus patrones.",
    alt: "Pestaña de preparar al rival con el plan de la próxima partida",
  },
  {
    shot: "ranking",
    title: "Ranking por rol",
    body: "Los mejores del mundo en cada rol, con sus cifras.",
    alt: "Tabla de ranking por rol con jugadores profesionales y sus cifras",
  },
  {
    shot: "lineups",
    title: "Lineups pro con Practicar en CS2",
    body: "Humos, flashes y molotovs del mapa, listos para probar.",
    alt: "Mapa de Mirage con los lineups profesionales y la opción de practicar en CS2",
  },
  {
    shot: "informe",
    title: "Informes en PDF",
    body: "Un informe de scouting completo para el coach.",
    alt: "Informe de scouting en PDF con el análisis del rival",
  },
] as const;

const ASSISTANT_POINTS = [
  "Studio incluye un servidor MCP para Claude y ChatGPT.",
  "Pídele un Short o un informe del rival en lenguaje normal.",
  "Unas 200 herramientas.",
  "Te pide permiso antes de grabar o borrar.",
  "Usa tu suscripción, sin API key.",
] as const;

const REQUIREMENTS = [
  "Windows 10 u 11 de 64 bits.",
  "Counter-Strike 2 instalado con Steam.",
  "HLAE oficial: Studio lo instala y lo actualiza solo.",
  "Una tarjeta gráfica dedicada para grabar con la mejor calidad.",
  "Alrededor de 1 GB libre en el disco.",
] as const;

const eyebrowClass = "font-mono text-xs font-medium uppercase tracking-[0.2em] text-primary";
const h2Class = "mt-3 font-display text-3xl font-semibold leading-tight text-balance text-fg sm:text-4xl";
const leadClass = "mt-4 text-base leading-relaxed text-pretty text-muted sm:text-lg";
const pointsClass = "mt-6 space-y-3 text-[15px] leading-relaxed text-muted";
const sectionClass = "mx-auto max-w-6xl scroll-mt-20 px-5 py-16 sm:px-8 md:py-28";

function Wordmark() {
  return (
    <span className="inline-flex items-center gap-2.5">
      <img
        src="/brand/cliphub-mark.svg"
        alt=""
        width={28}
        height={28}
        loading="lazy"
        decoding="async"
        className="size-7"
      />
      <span className="font-display text-base font-bold tracking-wide">
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

function SectionHead({ eyebrow, title, id }: { eyebrow: string; title: string; id: string }) {
  return (
    <>
      <p className={eyebrowClass}>{eyebrow}</p>
      <h2 id={id} className={h2Class}>
        {title}
      </h2>
    </>
  );
}

function Points({ items }: { items: readonly string[] }) {
  return (
    <ul className={pointsClass}>
      {items.map((item) => (
        <li key={item} className="flex gap-3">
          <span aria-hidden="true" className="mt-[0.65em] size-1.5 shrink-0 rounded-full bg-primary" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}

function Split({
  text,
  media,
  reverse = false,
}: {
  text: React.ReactNode;
  media: React.ReactNode;
  reverse?: boolean;
}) {
  return (
    <div className="grid items-center gap-10 md:grid-cols-2 md:gap-14">
      <Reveal className={`min-w-0 ${reverse ? "md:order-2" : ""}`}>{text}</Reveal>
      <Reveal className={`min-w-0 ${reverse ? "md:order-1" : ""}`} delay={90}>
        {media}
      </Reveal>
    </div>
  );
}

function Section({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <section id={id} aria-labelledby={`${id}-titulo`} className={sectionClass}>
      {children}
    </section>
  );
}

function Steps() {
  return (
    <ol className="mt-7 space-y-6">
      {STEPS.map((step, index) => (
        <li key={step.title} className="flex gap-4">
          <span className="grid size-9 shrink-0 place-items-center rounded-full border border-line bg-surface font-mono text-sm text-primary">
            {index + 1}
          </span>
          <div>
            <h3 className="font-display text-lg font-semibold text-fg">{step.title}</h3>
            <p className="mt-1 text-sm leading-relaxed text-muted">{step.body}</p>
          </div>
        </li>
      ))}
    </ol>
  );
}

function ShortMedia() {
  return (
    <div className="relative mx-auto w-full max-w-[22rem] md:max-w-none">
      <div className="md:absolute md:right-0 md:top-12 md:w-[78%]">
        <Shot
          name="short-producer"
          title="Short Producer"
          alt="Editor de Short de ClipHub Studio con las jugadas elegidas, la etiqueta ACE y la vista previa en el móvil"
        />
      </div>
      <div className="relative z-10 mx-auto mt-6 w-[62%] min-w-[12rem] max-w-[18rem] md:ml-0 md:mt-0 md:w-[58%]">
        <PhoneFrame>
          <SectionVideo
            name="short"
            aspect="9/16"
            label="Un Short de ClipHub: un ACE de CS2 grabado en vertical"
          />
        </PhoneFrame>
      </div>
    </div>
  );
}

function VideosMedia() {
  return (
    <div className="relative">
      <Shot
        name="mis-videos"
        title="Mis vídeos"
        alt="Lista de vídeos de ClipHub Studio con los estados en cola, grabando, en edición y listo"
      />
      <div className="mt-4 md:absolute md:-bottom-8 md:right-0 md:mt-0 md:w-[62%]">
        <Shot
          name="publicar"
          title="Publicar en YouTube"
          alt="Asistente de YouTube con el título, la descripción, las etiquetas y la mejor hora para publicar"
        />
      </div>
    </div>
  );
}

function AnalysisCard({
  title,
  body,
  shot,
  alt,
  delay,
}: {
  title: string;
  body: string;
  shot: string;
  alt: string;
  delay: number;
}) {
  return (
    <Reveal delay={delay} className="min-w-0">
      <article className="flex h-full flex-col gap-3 rounded-2xl border border-line bg-surface/70 p-3">
        <Shot name={shot} title={title} alt={alt} />
        <div className="px-1 pb-1">
          <h3 className="font-display text-base font-semibold text-fg">{title}</h3>
          <p className="mt-1 text-sm leading-relaxed text-muted">{body}</p>
        </div>
      </article>
    </Reveal>
  );
}

function AssistantChat() {
  return (
    <div className="rounded-2xl border border-line bg-surface p-4 shadow-xl shadow-black/30">
      <div className="flex items-center justify-between gap-3 border-b border-line pb-3">
        <span className="font-mono text-[11px] uppercase tracking-[0.16em] text-primary">Asistente MCP</span>
        <span className="flex items-center gap-1.5 font-mono text-[11px] text-muted">
          <span aria-hidden="true" className="size-1.5 rounded-full bg-primary" />
          En local
        </span>
      </div>
      <div className="mt-4 space-y-3 text-sm leading-relaxed">
        <p className="ml-auto max-w-[88%] rounded-2xl rounded-br-sm bg-surface-2 px-4 py-2.5 text-fg">
          Hazme un Short de mi ACE en la ronda 19 de Inferno.
        </p>
        <p className="max-w-[92%] rounded-2xl rounded-bl-sm border border-line bg-bg px-4 py-2.5 text-muted">
          Puedo grabar la ronda 19 desde tu POV en 9:16. ¿Te vale el tema Pulse y los cortes al ritmo?
        </p>
      </div>
    </div>
  );
}

export default function Home() {
  return (
    <>
      <header className="sticky top-0 z-40 border-b border-line bg-bg/80 backdrop-blur-md">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-5 py-3 sm:px-8">
          <a href="#inicio" className="rounded-lg">
            <Wordmark />
          </a>
          <nav aria-label="Secciones" className="hidden items-center gap-6 text-sm text-muted md:flex">
            {NAV.map((link) => (
              <a key={link.href} href={link.href} className="transition-colors hover:text-fg">
                {link.label}
              </a>
            ))}
          </nav>
          <a
            href={DOWNLOAD_URL}
            className="inline-flex min-h-10 items-center gap-2 rounded-lg bg-accent px-4 text-sm font-semibold text-accent-fg transition-[transform,filter] duration-150 ease-out hover:brightness-110 motion-safe:active:scale-[0.97]"
          >
            <Download className="size-4" aria-hidden="true" />
            Descargar
          </a>
        </div>
      </header>

      <main>
        <section id="inicio" aria-labelledby="inicio-titulo" className="hero-backdrop scroll-mt-20 border-b border-line">
          <div className="mx-auto max-w-6xl px-5 pb-16 pt-12 sm:px-8 md:pb-24 md:pt-20">
            <Split
              text={
                <>
                  <p className={eyebrowClass}>CLIPHUB STUDIO · CS2 EN LOCAL</p>
                  <h1
                    id="inicio-titulo"
                    className="mt-4 font-display text-4xl font-bold leading-[1.05] text-balance sm:text-5xl lg:text-[3.4rem]"
                  >
                    De la demo al vídeo, sin editar a mano
                  </h1>
                  <p className={leadClass}>
                    Studio lee tu demo de CS2, encuentra las mejores jugadas, las graba en el juego y monta el vídeo. Y
                    analiza la partida ronda a ronda.
                  </p>
                  <div className="mt-8">
                    <DownloadButton label="Descargar para Windows" />
                    <DownloadMeta />
                  </div>
                </>
              }
              media={
                <div className="overflow-hidden rounded-2xl border border-line bg-surface shadow-2xl shadow-black/40">
                  <div className="flex items-center gap-2 border-b border-line bg-surface-2 px-3 py-2">
                    <span aria-hidden="true" className="h-1.5 w-8 rounded-full bg-line" />
                    <span className="font-mono text-[11px] text-muted">ClipHub Studio · presentación</span>
                  </div>
                  <SectionVideo
                    name="hero"
                    aspect="16/9"
                    eager
                    label="Vídeo de presentación de ClipHub Studio: de la demo de CS2 al vídeo editado"
                  />
                </div>
              }
            />
            <Reveal delay={160}>
              <ul className="mt-14 grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-line bg-line sm:grid-cols-4">
                {FACTS.map((fact) => (
                  <li
                    key={fact}
                    className="bg-surface px-4 py-4 text-center font-mono text-[11px] uppercase tracking-wider text-muted sm:text-xs"
                  >
                    {fact}
                  </li>
                ))}
              </ul>
            </Reveal>
          </div>
        </section>

        <Section id="como-funciona">
          <Split
            reverse
            text={
              <>
                <SectionHead eyebrow="01 / CÓMO FUNCIONA" title="Cómo funciona" id="como-funciona-titulo" />
                <p className={leadClass}>
                  Un flujo de tres pasos: de tu archivo a un vídeo terminado, sin abrir un editor.
                </p>
                <Steps />
              </>
            }
            media={
              <div className="rounded-2xl border border-line bg-surface p-5 sm:p-6">
                <p className="font-mono text-[11px] uppercase tracking-wider text-muted">Así trabaja Studio</p>
                <ProductFlow />
              </div>
            }
          />
        </Section>

        <Section id="importar">
          <Split
            text={
              <>
                <SectionHead eyebrow="02 / IMPORTAR" title="Importa de donde juegues" id="importar-titulo" />
                <p className={leadClass}>Tus demos, tus partidas y los torneos que sigues, en el mismo sitio.</p>
                <Points items={SOURCES} />
                {/* slot: import-sources */}
              </>
            }
            media={
              <div className="space-y-4">
                <Shot
                  name="import"
                  title="Importar partida"
                  alt="Pantalla de importar de ClipHub Studio con la opción de elegir una demo o arrastrarla"
                />
                <div className="grid grid-cols-2 gap-4">
                  <Shot
                    name="partidas-pro"
                    title="Partidas pro"
                    alt="Listado de partidas profesionales recientes con sus equipos y torneos"
                  />
                  <Shot
                    name="faceit"
                    title="FACEIT"
                    alt="Jugadores de FACEIT seguidos, con su último resultado y sus partidas"
                  />
                </div>
              </div>
            }
          />
        </Section>

        <Section id="short">
          <Split
            reverse
            text={
              <>
                <SectionHead eyebrow="03 / SHORT" title="Short: tus mejores jugadas en vertical" id="short-titulo" />
                <p className={leadClass}>
                  Elige al jugador y Studio encuentra sus jugadas, las graba en CS2 con HLAE y las monta en un vídeo
                  listo para subir.
                </p>
                <Points items={SHORT_POINTS} />
              </>
            }
            media={<ShortMedia />}
          />
        </Section>

        <Section id="demo-completa">
          <Split
            text={
              <>
                <SectionHead
                  eyebrow="04 / DEMO COMPLETA"
                  title="Demo completa: la partida entera desde un jugador"
                  id="demo-completa-titulo"
                />
                <p className={leadClass}>La partida entera, desde un jugador, con la calidad de un directo.</p>
                <Points items={DEMO_POINTS} />
              </>
            }
            media={
              <div className="space-y-4">
                <div className="overflow-hidden rounded-xl border border-line bg-black shadow-xl shadow-black/30">
                  <SectionVideo
                    name="demo-completa"
                    aspect="16/9"
                    label="Demo completa de CS2 grabada desde un jugador con el HUD de retransmisión"
                  />
                </div>
                <Shot
                  name="demo-ajustes"
                  title="Ajustes de Demo completa"
                  alt="Ajustes de la demo completa con la vista previa del HUD, la intro, el sponsor y el outro"
                />
                {/* slot: hud-carousel */}
              </div>
            }
          />
        </Section>

        <Section id="streams">
          <Split
            reverse
            text={
              <>
                <SectionHead
                  eyebrow="05 / STREAMS"
                  title="Clips de stream: tus directos, en Shorts"
                  id="streams-titulo"
                />
                <p className={leadClass}>
                  Pega un enlace o sube un MP4. Studio encuentra tus kills y los momentos de voz y los convierte en
                  Shorts.
                </p>
                <Points items={STREAM_POINTS} />
              </>
            }
            media={
              <div className="space-y-4">
                <Shot
                  name="stream-editor"
                  title="Editor de clips"
                  alt="Editor de clips de stream con el vídeo original y la vista previa en 9:16"
                />
                {/* slot: stream-layouts */}
              </div>
            }
          />
        </Section>

        <Section id="mis-videos">
          <Split
            text={
              <>
                <SectionHead eyebrow="06 / MIS VÍDEOS" title="Mis vídeos y publicar" id="mis-videos-titulo" />
                <p className={leadClass}>Todo lo que has mandado a grabar o editar, con su estado a la vista.</p>
                <Points items={VIDEO_POINTS} />
              </>
            }
            media={<VideosMedia />}
          />
        </Section>

        <Section id="analisis">
          <Reveal className="max-w-3xl">
            <SectionHead eyebrow="07 / ANÁLISIS" title="Análisis: prepara cada partida" id="analisis-titulo" />
            <p className={leadClass}>
              Cada demo se analiza en local una sola vez y queda en tu biblioteca. Las partidas pro llegan ya
              analizadas.
            </p>
          </Reveal>
          {/* slot: feature-bento */}
          <div className="mt-10 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
            {ANALYSIS.map((item, index) => (
              <AnalysisCard key={item.shot} {...item} delay={(index % 4) * 60} />
            ))}
          </div>
          <Reveal delay={120}>
            <p className="mt-10 border-t border-line pt-6 text-sm leading-relaxed text-muted">
              También: simulación de veto, buscador de patrones, Libreta de jugadas, voz del equipo con TeamSpeak y
              transcripción local.
            </p>
          </Reveal>
        </Section>

        <Section id="asistentes">
          <Split
            text={
              <>
                <SectionHead eyebrow="08 / ASISTENTES" title="Con tu asistente de IA" id="asistentes-titulo" />
                <p className={leadClass}>Pídele a tu asistente lo que antes hacías a mano, en lenguaje normal.</p>
                <Points items={ASSISTANT_POINTS} />
              </>
            }
            media={
              <div className="space-y-4">
                {/* slot: assistant-chat */}
                <AssistantChat />
              </div>
            }
          />
        </Section>

        <Section id="descargar">
          <Split
            text={
              <>
                <SectionHead eyebrow="09 / DESCARGAR" title="Descarga ClipHub Studio" id="descargar-titulo" />
                <p className={leadClass}>
                  Las demos se analizan, graban y editan en tu PC: no necesitas cuenta y tus archivos no se suben a
                  ningún sitio.
                </p>
                <div className="mt-7">
                  <DownloadButton label="Descargar para Windows" />
                  <DownloadMeta />
                </div>
                <h3 className="mt-8 font-display text-lg font-semibold text-fg">Si Windows avisa al instalar</h3>
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
              </>
            }
            media={
              <div className="rounded-2xl border border-line bg-surface p-6">
                <h3 className="font-display text-lg font-semibold text-fg">Qué necesitas</h3>
                <Points items={REQUIREMENTS} />
              </div>
            }
          />
        </Section>
      </main>

      <footer className="border-t border-line px-5 py-10 sm:px-8">
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
