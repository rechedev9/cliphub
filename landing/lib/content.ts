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

export const PRODUCT_VIDEO = {
  src: "/film/producto",
  title: "Un rival, leído en un minuto.",
  lead: "Iberian Soul de principio a fin: sus jugadores, sus bombsites, su playbook y su ronda en el replay. Pantallas reales, nada simulado.",
  label:
    "Vídeo sin sonido de 62 segundos: ClipHub prepara un partido contra Iberian Soul con el rating de sus jugadores, sus bombsites, su playbook, sus estadísticas por lado y una ronda en el replay.",
} as const;

export const HERO = {
  kicker: "Análisis de demos de CS2 para equipos",
  title: "Lee al rival antes de que juegue.",
  lead: "Suelta sus demos y en minutos tienes sus bombsites, sus rutas, su playbook y su veto. Gratis, en el navegador o en Windows.",
  primary: "Abrir en el navegador",
  secondary: "Descargar para Windows",
  assurances: ["Gratis, sin tarjeta", "Tus demos no salen de tu ordenador", "FACEIT, Matchmaking y partidos pro"],
} as const;

export const STEPS = [
  {
    title: "Suelta las demos",
    text: "Archivos .dem, .dem.zst de FACEIT o .dem.gz, todas las que quieras a la vez. O abre partidos pro ya analizados.",
  },
  {
    title: "Tu navegador las lee",
    text: "El análisis se hace en tu ordenador y solo se guarda el resultado. La demo no se sube a ningún sitio.",
  },
  {
    title: "Prepara el partido",
    text: "Scouting, replay 2D, playbook, veto e informe para el equipo. Antes de que empiece la llamada.",
  },
] as const;

export const FAQ = [
  {
    q: "¿De verdad es gratis?",
    a: "Sí. Todo el análisis, los espacios de equipo y la app de Windows son gratis: partidos enteros, sin límite de rondas y sin pedir tarjeta.",
  },
  {
    q: "¿Se suben mis demos a algún servidor?",
    a: "No. En el navegador, ClipHub analiza cada demo en tu ordenador y solo guarda el análisis. En Windows, todo ocurre en tu PC.",
  },
  {
    q: "¿Qué demos puedo analizar?",
    a: "Demos de CS2 en .dem, .dem.zst (FACEIT) y .dem.gz, partidas de Matchmaking con su código para compartir y partidos pro de HLTV ya analizados por ClipHub.",
  },
  {
    q: "¿Necesito Windows?",
    a: "Para analizar, no: basta con el navegador. La app de Windows añade la grabación y el montaje de vídeo, que necesitan CS2 instalado.",
  },
  {
    q: "¿Puedo trabajar con mi equipo?",
    a: "Sí. Creas un espacio de equipo, invitas a jugadores y staff con un enlace y compartís biblioteca y libreta, en la web y en Studio.",
  },
  {
    q: "¿En qué se diferencia de otras herramientas?",
    a: "Lees el partido entero sin pagar, las comms de voz, los informes, el veto y el playbook vienen incluidos, y tus demos se quedan contigo.",
  },
] as const;

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
      src: "/shots/replay.webp",
      alt: "Replay 2D de la ronda 1 de Nemiga contra Iberian Soul en Mirage, con los diez jugadores sobre el radar, su vida, su dinero y la línea de tiempo de las 22 rondas.",
      caption: "Nemiga contra Iberian Soul en Mirage, ronda a ronda, con cada jugador sobre el radar.",
      callouts: [
        { value: "22", label: "Rondas en el replay" },
        { value: "10", label: "Jugadores en el radar" },
        { value: "8 Hz", label: "Posiciones por segundo" },
      ],
    },
  },
  {
    id: "bombsites",
    label: "Bombsites",
    shot: {
      src: "/shots/bombsites.webp",
      alt: "Tabla de bombsites de Iberian Soul en Mirage: defendiendo, les atacan A el 47 % de las veces; atacando A, plantan el 90 % y ganan el 60 % de las rondas.",
      caption: "Dónde ataca y dónde aguanta Iberian Soul, sitio a sitio y mapa a mapa.",
      callouts: [
        { value: "47 %", label: "Ataques rivales a A" },
        { value: "90 %", label: "Plantan cuando van a A" },
        { value: "60 %", label: "Ganan atacando A" },
      ],
    },
  },
  {
    id: "playbook",
    label: "Playbook",
    shot: {
      src: "/shots/playbook.webp",
      alt: "Playbook de Iberian Soul en Mirage de T: ganan el 75 % de sus ejecuciones y el 18 % de las rondas de control de mapa, en 23 rondas leídas.",
      caption: "Las tácticas que repite el rival, cuántas veces y cuánto le funcionan.",
      callouts: [
        { value: "75 %", label: "Ganan sus ejecuciones" },
        { value: "18 %", label: "Ganan con control de mapa" },
        { value: "23", label: "Rondas de T en Mirage" },
      ],
    },
  },
  {
    id: "preparar",
    label: "Preparar",
    shot: {
      src: "/shots/preparar.webp",
      alt: "Pantalla Preparar de Iberian Soul con la ficha de sausoL, mopoz, alex, ag1l y dav1g: rol, rating de ClipHub, K/D, ADR, KPR y DPR en 219 rondas.",
      caption: "Roles y forma de cada jugador del rival, antes de la llamada.",
      callouts: [
        { value: "1,20", label: "Rating de su entry" },
        { value: "219", label: "Rondas leídas" },
        { value: "11", label: "Mapas" },
      ],
    },
  },
  {
    id: "estadisticas",
    label: "Estadísticas",
    shot: {
      src: "/shots/estadisticas.webp",
      alt: "Estadísticas por lado de Iberian Soul: 51 % de rondas ganadas de T y 63 % de CT, con pistolas, primera baja, trades y utilidad frente a la media.",
      caption: "Cada cifra por lado, frente a la media de tu biblioteca.",
      callouts: [
        { value: "63 %", label: "Rondas ganadas de CT" },
        { value: "+10 pp", label: "Sobre la media" },
        { value: "82 %", label: "Pistolas ganadas de CT" },
      ],
    },
  },
  {
    id: "probabilidad",
    label: "Probabilidad",
    shot: {
      src: "/shots/probabilidad.webp",
      alt: "Probabilidad de ganar cada ronda de Nemiga contra Iberian Soul en Mirage y las bajas que más la movieron, con khaN matando a dav1g en la ronda 6.",
      caption: "Qué rondas se perdieron antes de empezar y qué bajas decidieron el mapa.",
      callouts: [
        { value: "+86,3 pp", label: "La baja que más pesó" },
        { value: "52 %", label: "Al empezar la ronda 1" },
        { value: "22", label: "Rondas analizadas" },
      ],
    },
  },
  {
    id: "pro",
    label: "Partidos pro",
    shot: {
      src: "/shots/partidas-pro.webp",
      alt: "Lista de partidos pro de HLTV analizados por ClipHub: series al mejor de tres con sus vetos, sus mapas y su torneo, listas para abrir en el replay.",
      caption: "Las series pro de los últimos días, con su veto, abiertas sin bajar la demo.",
      callouts: [
        { value: "0", label: "Demos que descargar" },
        { value: "BO3", label: "Series con su veto" },
        { value: "HLTV", label: "Partidos recientes" },
      ],
    },
  },
  {
    id: "lineups",
    label: "Lineups pro",
    shot: {
      src: "/shots/lineups.webp",
      alt: "Lineups pro por mapa: Mirage con 1107 lineups y 98 combos de 218 mapas pro, Dust2, Cache, Ancient, Nuke, Inferno y Anubis.",
      caption: "Las granadas que repiten los pros, leídas de sus demos.",
      callouts: [
        { value: "1107", label: "Lineups en Mirage" },
        { value: "98", label: "Combos" },
        { value: "7", label: "Mapas" },
      ],
    },
  },
  {
    id: "libreta",
    label: "Libreta",
    shot: {
      src: "/shots/libreta.webp",
      alt: "La libreta del equipo con 18 jugadas por mapa y lado: B empuja a Catwalk, control de Mid con humo desde base y otras, cada una con su estado.",
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
    { feature: "Tus demos", ours: "Se quedan en tu ordenador", theirs: "Se suben a su servidor" },
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
  { href: "#preguntas", label: "Preguntas" },
  { href: "#descargar", label: "Descargar" },
] as const;
