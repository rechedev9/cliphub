interface Crop {
  /** Center of the region to show, as a fraction of the capture's width and height. */
  x: number;
  y: number;
  zoom: number;
}

interface Tile {
  title: string;
  line: string;
  shot: string;
  alt: string;
  crop: Crop;
}

const LARGE_TILES: readonly Tile[] = [
  {
    title: "Replay 2D",
    line: "Cada ronda sobre el mapa, con granadas, recorridos y voz del equipo.",
    shot: "/media/shots/replay.webp",
    alt: "Replay 2D de Mirage con los recorridos y las granadas de ambos equipos.",
    crop: { x: 0.57, y: 0.49, zoom: 1.35 },
  },
  {
    title: "Mapa de calor",
    line: "Dónde juega cada equipo, por lado, compra y mapa.",
    shot: "/media/shots/heatmap.webp",
    alt: "Mapa de calor de Mirage con las zonas donde más juega Team Vitality.",
    crop: { x: 0.58, y: 0.68, zoom: 1.4 },
  },
];

const SMALL_TILES: readonly Tile[] = [
  {
    title: "Resumen de la partida",
    line: "Ronda a ronda: quién ganó, cómo y con qué economía.",
    shot: "/media/shots/partida.webp",
    alt: "Resumen de la partida con el marcador y el resultado de cada ronda.",
    crop: { x: 0.44, y: 0.55, zoom: 1.9 },
  },
  {
    title: "Equipos",
    line: "Plantilla, lados, bombsites y su rating de ClipHub frente a la media.",
    shot: "/media/shots/equipo.webp",
    alt: "Estadísticas de Team Vitality por lado comparadas con la media del grupo.",
    crop: { x: 0.5, y: 0.47, zoom: 1.8 },
  },
  {
    title: "Preparar al rival",
    line: "Primeras muertes, defaults, setups y avisos de datos.",
    shot: "/media/shots/preparar.webp",
    alt: "Preparación del rival con los ratings de los cinco jugadores.",
    crop: { x: 0.4, y: 0.48, zoom: 2 },
  },
  {
    title: "Ranking por rol",
    line: "Quién juega mejor cada rol en cada mapa, entre los pros.",
    shot: "/media/shots/ranking.webp",
    alt: "Ranking de jugadores profesionales con rating, ADR, KAST y aperturas.",
    crop: { x: 0.45, y: 0.66, zoom: 1.8 },
  },
  {
    title: "Lineups pro",
    line: "Las granadas que repiten los pros, y Practicar las abre en CS2.",
    shot: "/media/shots/lineups.webp",
    alt: "Mapa de Mirage con los destinos de los lineups profesionales.",
    crop: { x: 0.45, y: 0.55, zoom: 1.5 },
  },
  {
    title: "Informes",
    line: "Scouting listo para el coach, en HTML o PDF.",
    shot: "/media/shots/informe.webp",
    alt: "Informe de Vitality en Mirage e Inferno con claves y plan de partida.",
    crop: { x: 0.5, y: 0.45, zoom: 1.8 },
  },
];

const EXTRAS = [
  "Simulación de veto",
  "Buscador de patrones",
  "Libreta de jugadas",
  "Voz del equipo con TeamSpeak",
  "Transcripción local",
] as const;

/** Offset that centers the crop, clamped so the capture always covers the frame. */
function cropOffset(center: number, zoom: number) {
  const offset = (0.5 - center * zoom) * 100;
  return `${Math.min(0, Math.max((1 - zoom) * 100, offset))}%`;
}

function ShotTile({ tile, large }: { tile: Tile; large: boolean }) {
  const { x, y, zoom } = tile.crop;
  return (
    <article
      className={`flex flex-col overflow-hidden rounded-xl border border-line bg-surface transition-colors duration-200 [@media(hover:hover)]:hover:border-white/25 ${
        large ? "lg:col-span-3" : "lg:col-span-2"
      }`}
    >
      <div className="px-5 pb-4 pt-5">
        <h3 className={`font-display font-semibold tracking-tight text-fg ${large ? "text-2xl" : "text-lg"}`}>
          {tile.title}
        </h3>
        <p className="mt-1.5 text-sm leading-relaxed text-muted">{tile.line}</p>
      </div>
      <div className="relative mt-auto aspect-[16/10] overflow-hidden border-t border-line">
        <img
          src={tile.shot}
          alt={tile.alt}
          width={1440}
          height={900}
          loading="lazy"
          decoding="async"
          className="absolute max-w-none"
          style={{
            width: `${zoom * 100}%`,
            height: `${zoom * 100}%`,
            left: cropOffset(x, zoom),
            top: cropOffset(y, zoom),
          }}
        />
      </div>
    </article>
  );
}

export default function FeatureBento() {
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-6">
        {LARGE_TILES.map((tile) => (
          <ShotTile key={tile.title} tile={tile} large />
        ))}
        {SMALL_TILES.map((tile) => (
          <ShotTile key={tile.title} tile={tile} large={false} />
        ))}
      </div>
      <ul aria-label="Otras herramientas de análisis" className="flex flex-wrap gap-2">
        {EXTRAS.map((extra) => (
          <li key={extra} className="rounded-full border border-line bg-surface px-3.5 py-1.5 text-sm text-muted">
            {extra}
          </li>
        ))}
      </ul>
    </div>
  );
}
