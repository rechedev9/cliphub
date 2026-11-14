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

export interface Film {
  src: string;
  label: string;
}

export const FILMS = {
  hero: { src: "/film/mapa", label: "Radar de Mirage con los jugadores de los dos equipos moviéndose durante una ronda" },
  duels: { src: "/film/duelos", label: "Duelos de una ronda en Mirage marcados sobre el radar" },
  team: { src: "/film/equipo", label: "Un equipo avanzando por Mirage, con cada jugador marcado sobre el radar" },
  pro: { src: "/film/pro", label: "Ronda de un partido pro en Mirage grabada en primera persona con el HUD de ClipHub" },
} as const satisfies Record<string, Film>;

export const HERO = {
  kicker: "Análisis de CS2 · Gratis",
  title: "Lee al rival antes de que juegue.",
  lead: "Replay 2D, scouting, comms de voz y la libreta del equipo. En el navegador o en Windows.",
  primary: "Abrir en el navegador",
  secondary: "Descargar para Windows",
} as const;

export const STATS = [
  { value: "100+", label: "Usuarios" },
  { value: "130+", label: "Versiones publicadas" },
  { value: "0 €", label: "Planes de pago" },
] as const;

export const VIEWER_TABS: readonly ViewerTab[] = [
  {
    id: "replay",
    label: "Replay",
    shot: {
      src: "/shots/partida-replay-heatmap.webp",
      alt: "Replay 2D de Cache con el mapa de calor de posiciones de FUZOS en el lado T, y la lista de zonas con más actividad.",
      caption: "Mapa de calor de un equipo en Cache, filtrado por lado y tramo de ronda.",
      callouts: [
        { value: "2747", label: "Posiciones en 15 rondas" },
        { value: "33", label: "Muestras en Outside A" },
        { value: "4", label: "Tramos de ronda" },
      ],
    },
  },
  {
    id: "recorridos",
    label: "Recorridos",
    shot: {
      src: "/shots/equipo-replay-patrones.webp",
      alt: "Recorridos de ataque de Los Galacticos en Cache superpuestos sobre el radar, con el buscador de patrones y la lista de rondas.",
      caption: "Todas las rutas de ataque de un equipo, y las rondas que cumplen un patrón.",
      callouts: [
        { value: "21/21", label: "Rondas con el patrón" },
        { value: "57 %", label: "De ellas, ganadas" },
        { value: "Caja", label: "Dibuja una zona y busca" },
      ],
    },
  },
  {
    id: "bombsites",
    label: "Bombsites",
    shot: {
      src: "/shots/equipo-bombsites.webp",
      alt: "Tabla de bombsites de Los Galacticos: en Cache defendiendo, les atacan A el 56 % de las veces; atacando, ganan el 75 % de las rondas en A.",
      caption: "Dónde ataca y dónde aguanta un rival, sitio a sitio.",
      callouts: [
        { value: "56 %", label: "Ataques rivales a A" },
        { value: "75 %", label: "Ganadas atacando A" },
        { value: "30 %", label: "Retakes ganados en A" },
      ],
    },
  },
  {
    id: "preparar",
    label: "Preparar",
    shot: {
      src: "/shots/equipo-preparar.webp",
      alt: "Pantalla Preparar de Los Galacticos con la ficha de cada jugador: rol, rating de ClipHub, K/D, ADR, KPR y DPR en 107 rondas.",
      caption: "Roles y forma de cada jugador del rival, mapa a mapa.",
      callouts: [
        { value: "1,31", label: "Rating del entry" },
        { value: "107", label: "Rondas leídas" },
        { value: "5", label: "Mapas" },
      ],
    },
  },
  {
    id: "estadisticas",
    label: "Estadísticas",
    shot: {
      src: "/shots/equipo-estadisticas.webp",
      alt: "Estadísticas por lado de Los Galacticos: 59 % de rondas ganadas de T y 57 % de CT, con pistolas, primera baja y trades frente a la media.",
      caption: "Cada cifra por lado, frente a la media de tu biblioteca.",
      callouts: [
        { value: "59 %", label: "Rondas ganadas de T" },
        { value: "+15 pp", label: "Sobre la media" },
        { value: "57 %", label: "Rondas ganadas de CT" },
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
        { value: "+82,9 pp", label: "La baja que más pesó" },
        { value: "49 %", label: "Al empezar la ronda 1" },
        { value: "15", label: "Rondas analizadas" },
      ],
    },
  },
  {
    id: "lineups",
    label: "Lineups pro",
    shot: {
      src: "/shots/lineups.webp",
      alt: "Lineups pro por mapa: Mirage con 765 lineups y 70 combos, Dust2 con 749, Ancient con 683, Cache, Nuke, Inferno y Anubis.",
      caption: "Las granadas que repiten los pros, leídas de sus demos.",
      callouts: [
        { value: "765", label: "Lineups en Mirage" },
        { value: "70", label: "Combos" },
        { value: "7", label: "Mapas" },
      ],
    },
  },
  {
    id: "libreta",
    label: "Libreta",
    shot: {
      src: "/shots/libreta.webp",
      alt: "La libreta del equipo con 18 jugadas por mapa y lado: B empuja a Catwalk, control de Mid con humo desde base y otras.",
      caption: "El stratbook del equipo, compartido con todo el staff.",
      callouts: [
        { value: "18", label: "Jugadas" },
        { value: "2", label: "Mapas" },
        { value: "1", label: "Libreta para todos" },
      ],
    },
  },
];

export const CAPABILITIES = [
  { title: "Scouting", text: "Rondas ganadas, pistolas, primera baja, trades y utilidad, por mapa y lado." },
  { title: "Bombsites", text: "Qué sitio atacan, cuánto plantan y cuántos retakes ganan." },
  { title: "Playbook", text: "Las jugadas que el rival repite, detectadas y abiertas en el replay." },
  { title: "Veto", text: "BO1, BO3 o BO5 con el pool actual, antes de la llamada." },
  { title: "Informes", text: "Scouting para el coach y el IGL en HTML o PDF." },
  { title: "Partidos pro", text: "Los equipos de primer nivel, con las mismas herramientas." },
] as const;

export const COMMS = {
  title: "Escucha la ronda.",
  lead: "ClipHub extrae la voz del equipo, la transcribe y la sincroniza con el replay. Buscas una frase y saltas al segundo en que se dijo.",
  sample: [
    { clock: "1:52", speaker: "IGL", phase: "Freezetime", text: "Default lento, nadie pisa A hasta el minuto." },
    { clock: "1:08", speaker: "Entry", phase: "Ronda", text: "Dos en long, uno con AWP." },
    { clock: "0:41", speaker: "IGL", phase: "Ronda", text: "Rotamos B, humo a CT y entramos." },
    { clock: "0:29", speaker: "Soporte", phase: "Post-plant", text: "Molly en default, aguantad el retake." },
  ],
} as const;

export const TEAM_POINTS = [
  "Espacios de equipo con invitaciones",
  "Libreta compartida: jugadas, roles, llamadas y callouts",
  "Web y escritorio sincronizados",
] as const;

export const COMPARISON = {
  ours: "ClipHub",
  theirs: "Otras herramientas",
  rows: [
    { feature: "Precio", ours: "Gratis", theirs: "Suscripción mensual" },
    { feature: "Partido completo sin pagar", ours: "Todas las rondas", theirs: "A menudo, media partida" },
    { feature: "Comms de voz transcritas", ours: "Incluido", theirs: "Plan de organización" },
    { feature: "Informes en PDF", ours: "Incluido", theirs: "Depende" },
    { feature: "Simulador de veto", ours: "Incluido", theirs: "Rara vez" },
    { feature: "Playbook detectado", ours: "Incluido", theirs: "Rara vez" },
    { feature: "Equipos con invitaciones", ours: "Gratis", theirs: "De pago por asiento" },
    { feature: "Vídeo desde la demo", ours: "Shorts y partido entero", theirs: "No" },
  ],
} as const;

export const CREATOR_POINTS = [
  { title: "Shorts 9:16", text: "Las jugadas de un jugador, grabadas en CS2 y montadas a ritmo de música." },
  { title: "Demo completa", text: "El partido entero desde un jugador, con HUD de retransmisión." },
  { title: "Clips de stream", text: "De Twitch, YouTube o Kick a vertical con la facecam encuadrada." },
] as const;

export const REQUIREMENTS = [
  "Windows 10 u 11 de 64 bits",
  "Counter-Strike 2 con Steam, para grabar vídeo",
  "HLAE oficial: Studio lo instala solo",
  "Gráfica dedicada para la mejor calidad",
  "Alrededor de 1 GB libre",
] as const;

export const NAV = [
  { href: "#analisis", label: "Análisis" },
  { href: "#comms", label: "Comms" },
  { href: "#equipo", label: "Equipo" },
  { href: "#creadores", label: "Creadores" },
  { href: "#descargar", label: "Descargar" },
] as const;
