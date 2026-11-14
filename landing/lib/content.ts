export interface Shot {
  src: string;
  alt: string;
  caption: string;
  callouts: readonly { value: string; label: string }[];
}

export interface ViewerTab {
  id: string;
  label: string;
  shot: Shot;
}

export const HERO = {
  kicker: "Análisis de CS2 para equipos",
  title: "Prepara cada partido con todo lo que hay en la demo.",
  lead: "Replay 2D, scouting de rivales, comms de voz y la libreta del equipo. En el navegador o en Windows, con la misma biblioteca. Gratis.",
  primary: "Abrir en el navegador",
  secondary: "Descargar para Windows",
} as const;

export const VIEWER_TABS: readonly ViewerTab[] = [
  {
    id: "replay",
    label: "Replay",
    shot: {
      src: "/shots/partida-replay-heatmap.webp",
      alt: "Replay 2D de Cache con el mapa de calor de posiciones de FUZOS en el lado T, y la lista de zonas con más actividad.",
      caption: "Mapa de calor de un equipo en Cache, filtrado por lado y tramo de ronda.",
      callouts: [
        { value: "2747", label: "posiciones en 15 rondas" },
        { value: "Outside A", label: "zona más activa, 33 muestras" },
        { value: "1:00 a 0:40", label: "filtra cualquier tramo de la ronda" },
      ],
    },
  },
  {
    id: "recorridos",
    label: "Recorridos",
    shot: {
      src: "/shots/equipo-replay-patrones.webp",
      alt: "Recorridos de ataque de Los Galacticos en Cache superpuestos sobre el radar, con el buscador de patrones y la lista de rondas.",
      caption: "Todas las rutas de ataque de un equipo en un mapa, y las rondas que cumplen un patrón.",
      callouts: [
        { value: "21 de 21", label: "rondas encajan con el patrón" },
        { value: "57 %", label: "de esas rondas las ganan" },
        { value: "Caja", label: "dibuja una zona y busca quién pasa" },
      ],
    },
  },
  {
    id: "bombsites",
    label: "Bombsites",
    shot: {
      src: "/shots/equipo-bombsites.webp",
      alt: "Tabla de bombsites de Los Galacticos: en Cache defendiendo, les atacan A el 56 % de las veces; atacando, ganan el 75 % de las rondas en A.",
      caption: "Dónde ataca y dónde aguanta un rival, sitio a sitio y mapa a mapa.",
      callouts: [
        { value: "56 %", label: "de los ataques rivales van a A en Cache" },
        { value: "75 %", label: "rondas ganadas cuando atacan A" },
        { value: "30 %", label: "retakes ganados en A" },
      ],
    },
  },
  {
    id: "preparar",
    label: "Preparar",
    shot: {
      src: "/shots/equipo-preparar.webp",
      alt: "Pantalla Preparar de Los Galacticos con la ficha de cada jugador: rol, rating de ClipHub, K/D, ADR, KPR y DPR en 107 rondas.",
      caption: "El rival de un vistazo: roles, forma de cada jugador y cómo juegan cada mapa.",
      callouts: [
        { value: "1,31", label: "rating del entry, el más alto del equipo" },
        { value: "107", label: "rondas leídas en 5 mapas" },
        { value: "1 clic", label: "y se guarda como informe" },
      ],
    },
  },
  {
    id: "probabilidad",
    label: "Probabilidad",
    shot: {
      src: "/shots/partida-metricas.webp",
      alt: "Gráfico de probabilidad de ganar cada ronda de Early Vacation contra FUZOS y las bajas que más movieron la probabilidad.",
      caption: "Qué rondas se perdieron antes de empezar y qué bajas decidieron el partido.",
      callouts: [
        { value: "+82,9 pp", label: "la baja que más cambió una ronda" },
        { value: "49 %", label: "probabilidad al empezar la ronda 1" },
        { value: "15", label: "rondas, cada una con su gráfico" },
      ],
    },
  },
];

export const PREP_POINTS = [
  { title: "Scouting por mapa y lado", text: "Rondas ganadas, pistolas, primera baja, trades y utilidad, comparados con la media de tu biblioteca." },
  { title: "Bombsites y retakes", text: "Qué sitio atacan, cuánto plantan y cuántos retakes ganan, con las rondas detrás de cada número." },
  { title: "Playbook detectado", text: "ClipHub encuentra las jugadas que el rival repite y te deja abrir cada una en el replay." },
  { title: "Simulador de veto", text: "BO1, BO3 o BO5 con el pool actual, para decidir qué banear antes de la llamada." },
  { title: "Informes de scouting", text: "Un informe para el coach y el IGL, en HTML o PDF, a partir de las partidas que elijas." },
  { title: "Lineups pro", text: "Humos, flashes y molotovs que los pros repiten, leídos de sus demos y agrupados por mapa." },
] as const;

export const PREP_SHOTS: readonly Shot[] = [
  {
    src: "/shots/equipo-estadisticas.webp",
    alt: "Estadísticas por lado de Los Galacticos: 59 % de rondas ganadas de T y 57 % de CT, con pistolas, primera baja y trades frente a la media.",
    caption: "Estadísticas por lado frente a la media de la biblioteca.",
    callouts: [
      { value: "59 %", label: "rondas ganadas de T, +15 pp" },
      { value: "57 %", label: "rondas ganadas de CT" },
    ],
  },
  {
    src: "/shots/lineups.webp",
    alt: "Lineups pro por mapa: Mirage con 765 lineups y 70 combos, Dust2 con 749, Ancient con 683, Cache, Nuke, Inferno y Anubis.",
    caption: "Miles de lineups de partidos pro, por mapa.",
    callouts: [
      { value: "765", label: "lineups en Mirage" },
      { value: "70", label: "combos de granadas" },
    ],
  },
];

export const COMMS = {
  title: "Escucha cómo se habló cada ronda",
  lead: "ClipHub extrae la voz del equipo, la transcribe y la sincroniza con el replay. Buscas una frase y saltas al segundo exacto en que se dijo.",
  points: [
    "Tiempo de habla de cada jugador por fase: freezetime, ronda, post-plant y muerto.",
    "Buscador de frases en todos los partidos transcritos.",
    "Cada frase abre el replay en ese momento.",
  ],
  sample: [
    { clock: "1:52", phase: "Freezetime", speaker: "IGL", text: "Default lento, nadie pisa A hasta el minuto." },
    { clock: "1:08", phase: "Ronda", speaker: "Entry", text: "Dos en long, uno con AWP." },
    { clock: "0:41", phase: "Ronda", speaker: "IGL", text: "Rotamos B, humo a CT y entramos." },
    { clock: "0:29", phase: "Post-plant", speaker: "Soporte", text: "Molly en default, aguantad el retake." },
  ],
} as const;

export const TEAM_POINTS = [
  { title: "Espacios de equipo", text: "Crea el equipo e invita a jugadores y staff con un enlace." },
  { title: "Libreta compartida", text: "Jugadas, lineups, roles, llamadas y callouts en un mismo sitio para todos." },
  { title: "Web y escritorio sincronizados", text: "Lo que analizas en el navegador aparece en Studio, y al revés." },
  { title: "Partidos pro incluidos", text: "Estudia a equipos de primer nivel con las mismas herramientas que usas para los tuyos." },
] as const;

export const TEAM_SHOT: Shot = {
  src: "/shots/libreta.webp",
  alt: "La libreta del equipo con 18 jugadas por mapa y lado: B empuja a Catwalk, control de Mid con humo desde base y otras.",
  caption: "La libreta: el stratbook del equipo, con filtros por mapa, lado y tipo de jugada.",
  callouts: [{ value: "18", label: "jugadas en la libreta" }],
};

export const COMPARISON = {
  ours: "ClipHub",
  theirs: "Otras herramientas",
  rows: [
    { feature: "Precio", ours: "Gratis", theirs: "Suscripción mensual" },
    { feature: "Partido completo sin pagar", ours: "Sí, todas las rondas", theirs: "A menudo solo la primera mitad" },
    { feature: "Comms de voz transcritas", ours: "Incluido", theirs: "Solo en planes de organización" },
    { feature: "Informes de scouting en PDF", ours: "Incluido", theirs: "Depende de la herramienta" },
    { feature: "Simulador de veto", ours: "Incluido", theirs: "Rara vez" },
    { feature: "Playbook detectado", ours: "Incluido", theirs: "Rara vez" },
    { feature: "Equipos con invitaciones", ours: "Gratis", theirs: "De pago, por asiento o por organización" },
    { feature: "Vídeo a partir de la demo", ours: "Shorts y partido completo", theirs: "No" },
  ],
} as const;

export const CREATOR_POINTS = [
  { title: "Shorts en 9:16", text: "Eliges al jugador y ClipHub encuentra sus jugadas, las graba en CS2 y las monta con cortes a ritmo de música." },
  { title: "Demo completa", text: "El partido entero desde un jugador, con HUD de retransmisión, transiciones e intro y outro propios." },
  { title: "Clips de stream", text: "De Twitch, YouTube, Kick o un MP4 a clips verticales con la facecam encuadrada." },
] as const;

export const REQUIREMENTS = [
  "Windows 10 u 11 de 64 bits.",
  "Counter-Strike 2 instalado con Steam, para grabar vídeo.",
  "HLAE oficial: Studio lo instala y lo actualiza solo.",
  "Una tarjeta gráfica dedicada para grabar con la mejor calidad.",
  "Alrededor de 1 GB libre en el disco.",
] as const;

export const STATS = [
  { value: "100+", label: "usuarios" },
  { value: "130+", label: "versiones publicadas" },
  { value: "0 €", label: "sin planes de pago" },
] as const;

export const NAV = [
  { href: "#analisis", label: "Análisis" },
  { href: "#comms", label: "Comms" },
  { href: "#equipo", label: "Equipo" },
  { href: "#creadores", label: "Creadores" },
  { href: "#descargar", label: "Descargar" },
] as const;
