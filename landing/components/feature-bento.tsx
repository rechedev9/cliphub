const LARGE_SHOTS = [
  {
    title: "Replay 2D",
    line: "Cada ronda sobre el mapa, con granadas, recorridos y voz del equipo.",
    shot: "/media/shots/replay.webp",
    focus: "50% 50%",
    alt: "Replay 2D de Mirage con los recorridos y las granadas de ambos equipos.",
    placement: "",
  },
  {
    title: "Mapa de calor",
    line: "Dónde juega cada equipo, por lado, compra y mapa.",
    shot: "/media/shots/heatmap.webp",
    focus: "47% 62%",
    alt: "Mapa de calor de Mirage con las zonas donde más juega Team Vitality.",
    placement: "",
  },
] as const;

const SHOTS = [
  {
    title: "Resumen de la partida",
    line: "Ronda a ronda: quién ganó, cómo y con qué economía.",
    shot: "/media/shots/partida.webp",
    focus: "56% 52%",
    alt: "Resumen de la partida con el marcador y el resultado de cada ronda.",
    placement: "",
  },
  {
    title: "Equipos",
    line: "Plantilla, lados, bombsites y su rating de ClipHub frente a la media.",
    shot: "/media/shots/equipo.webp",
    focus: "60% 55%",
    alt: "Estadísticas de Team Vitality por lado comparadas con la media del grupo.",
    placement: "",
  },
  {
    title: "Preparar al rival",
    line: "Primeras muertes, defaults, setups y avisos de datos.",
    shot: "/media/shots/preparar.webp",
    focus: "55% 45%",
    alt: "Preparación del rival con los ratings de los cinco jugadores.",
    placement: "",
  },
  {
    title: "Ranking por rol",
    line: "Quién juega mejor cada rol en cada mapa, entre los pros.",
    shot: "/media/shots/ranking.webp",
    focus: "57% 55%",
    alt: "Ranking de jugadores profesionales con rating, ADR, KAST y aperturas.",
    placement: "",
  },
  {
    title: "Lineups pro",
    line: "Las granadas que repiten los pros, y Practicar las abre en CS2.",
    shot: "/media/shots/lineups.webp",
    focus: "53% 55%",
    alt: "Mapa de Mirage con los destinos de los lineups profesionales.",
    placement: "lg:col-start-2",
  },
  {
    title: "Informes",
    line: "Scouting listo para el coach, en HTML o PDF.",
    shot: "/media/shots/informe.webp",
    focus: "52% 45%",
    alt: "Informe de Vitality en Mirage e Inferno con claves y plan de partida.",
    placement: "",
  },
] as const;

const EXTRAS = [
  "Simulación de veto",
  "Buscador de patrones",
  "Libreta de jugadas",
  "Voz del equipo con TeamSpeak",
  "Transcripción local",
] as const;

interface ShotTileProps {
  tile: (typeof LARGE_SHOTS)[number] | (typeof SHOTS)[number];
  large: boolean;
}

function ShotTile({ tile, large }: ShotTileProps) {
  const card = large
    ? "md:col-span-2 lg:row-span-2 lg:min-h-[32rem]"
    : tile.placement;
  const media = large
    ? "aspect-[16/9] lg:aspect-auto lg:min-h-0 lg:flex-1"
    : "aspect-[3/2]";

  return (
    <article
      className={`group flex flex-col overflow-hidden rounded-xl border border-line bg-surface transition-colors duration-300 [@media(hover:hover)]:hover:border-white/25 ${card}`}
    >
      <div className="px-5 pt-5 pb-4">
        <h3
          className={`font-display font-semibold tracking-tight text-fg ${
            large ? "text-2xl" : "text-lg"
          }`}
        >
          {tile.title}
        </h3>
        <p className="mt-1.5 text-sm leading-relaxed text-muted">{tile.line}</p>
      </div>
      <div className={`relative mt-auto overflow-hidden ${media}`}>
        <img
          src={tile.shot}
          alt={tile.alt}
          width={1440}
          height={900}
          loading="lazy"
          decoding="async"
          style={{ objectPosition: tile.focus }}
          className="absolute inset-0 size-full object-cover transition-transform duration-[400ms] ease-[cubic-bezier(0.23,1,0.32,1)] [@media(hover:hover)]:group-hover:scale-[1.04] motion-reduce:transition-none"
        />
      </div>
    </article>
  );
}

export default function FeatureBento() {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-4">
        {LARGE_SHOTS.map((tile) => (
          <ShotTile key={tile.title} tile={tile} large />
        ))}
        {SHOTS.map((tile) => (
          <ShotTile key={tile.title} tile={tile} large={false} />
        ))}
      </div>
      <ul aria-label="Otras herramientas de análisis" className="flex flex-wrap gap-2">
        {EXTRAS.map((extra) => (
          <li
            key={extra}
            className="rounded-full border border-line bg-surface px-3 py-1.5 font-mono text-[11px] tracking-[0.08em] text-muted uppercase"
          >
            {extra}
          </li>
        ))}
      </ul>
    </div>
  );
}
