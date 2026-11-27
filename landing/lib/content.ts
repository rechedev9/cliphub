export interface Film {
  src: string;
  label: string;
}

export interface Feature {
  title: string;
  text: string;
}

export const POV_FILM: Film = {
  src: "/film/pro",
  label: "Ronda de un partido pro en Mirage grabada en primera persona con el HUD de ClipHub",
};

export const HERO = {
  kicker: "Clipping de CS2 para creadores · Windows",
  title: "Tu demo, hecha vídeo.",
  lead: "Studio graba tus jugadas dentro de CS2 y monta el Short o el partido entero por ti, con kill feed, música, radar y tu marca. Sin abrir un editor.",
  primary: "Descargar para Windows",
  secondary: "Ver qué hace",
  assurances: ["Gratis, sin tarjeta", "Todo se queda en tu PC", "Demos de FACEIT, Matchmaking y pro"],
} as const;

export const PHONES = [
  { badge: "Triple kill", position: "35% 50%", startAt: 0 },
  { badge: "Clutch 1v3", position: "50% 50%", startAt: 2.7 },
  { badge: "Ace", position: "62% 50%", startAt: 5.3 },
] as const;

export const TICKER = [
  "Auto-Shorts",
  "Demo completa",
  "Clips de stream",
  "Kill feed centrado",
  "Radar en vivo",
  "Replay de la víctima",
  "Tarjeta de stats",
  "Subtítulos de voz",
  "Tu logo y tu sponsor",
] as const;

export const STATS = [
  { value: "10", label: "Shorts de una demo con un clic", side: "ct" },
  { value: "9:16", label: "Vertical para Shorts, TikTok y Reels", side: "t" },
  { value: "16:9", label: "Horizontal para YouTube", side: "ct" },
  { value: "0 €", label: "Sin planes de pago", side: "t" },
] as const;

export const STEPS: readonly Feature[] = [
  {
    title: "Importa la demo",
    text: "Arrastra un .dem, un .dem.zst de FACEIT o un .dem.gz, pega un código de Matchmaking o abre un partido pro.",
  },
  {
    title: "Elige las jugadas",
    text: "Studio puntúa cada jugada por bajas, clutch y multikills. Eliges tú, o Auto-Shorts saca la mejor de cada ronda.",
  },
  {
    title: "Studio graba y monta",
    text: "Abre CS2, graba con HLAE y monta el vídeo con música, efectos y tu marca. Tú solo lo publicas.",
  },
];

export const SHORT_FEATURES: readonly Feature[] = [
  { title: "Badges de jugada", text: "Clutch, ace y multikills marcados en pantalla en el momento justo." },
  { title: "Kill feed centrado", text: "Las bajas en grande y en el centro, legibles en un móvil." },
  { title: "HUD vertical", text: "Marcador compacto, reloj, bomba y vivos de cada equipo, hecho para 9:16." },
  { title: "Radar de ClipHub", text: "El mapa, cada jugador, humos, molotovs y la bomba, leídos de la demo." },
  { title: "Replay de la víctima", text: "La baja repetida desde los ojos del rival, a cámara lenta si quieres." },
  { title: "Tarjeta de stats", text: "Cierra el Short con el K/D, el ADR, el HS % y el rating del jugador." },
  { title: "Nombres y granadas", text: "La vida de cada compañero y de los rivales vistos, y la trayectoria de vuestras granadas." },
  { title: "Música y voz", text: "Música con su volumen, el volumen del juego y las comms del equipo subtituladas." },
];

export const FULL_DEMO = {
  title: "El partido entero, como en la tele.",
  lead: "Demo completa graba todas las rondas desde un jugador y las monta en un vídeo largo para YouTube, listo para subir.",
  points: [
    { title: "HUD de retransmisión", text: "Un HUD propio con el marcador y los equipos en lugar del de CS2." },
    { title: "Probabilidad de ganar", text: "Tras la planta, la opción de cada equipo de llevarse la ronda." },
    { title: "Radar en la esquina", text: "El radar de ClipHub en cada ronda, con humos, fuegos y la bomba." },
    { title: "Tu canal en pantalla", text: "Tu logo como marca de agua y el banner de tu sponsor cuando tú digas." },
  ],
} as const satisfies { title: string; lead: string; points: readonly Feature[] };

export const STREAMS = {
  title: "Tu stream, en clips.",
  lead: "Pega el enlace de un VOD de Twitch, Kick o YouTube, o sube un .mp4. Studio encuentra las jugadas y te las da en vertical, con la facecam encuadrada.",
  points: [
    { title: "Jugadas encontradas", text: "Studio recorre el VOD y te lista las jugadas para que elijas." },
    { title: "Vertical con facecam", text: "El gameplay y tu cámara en 9:16, sin recortar a mano." },
    { title: "Resumen horizontal", text: "Las mejores jugadas del stream en un solo vídeo 16:9." },
    { title: "Subtítulos", text: "Transcribe lo que dices y lo corriges antes de renderizar." },
  ],
} as const satisfies { title: string; lead: string; points: readonly Feature[] };

export const FAQ = [
  {
    q: "¿Es gratis?",
    a: "Sí. Studio es gratis: Shorts, demo completa y clips de stream, sin tarjeta y sin cuenta.",
  },
  {
    q: "¿Necesito tener CS2?",
    a: "Para grabar desde una demo, sí. Studio abre CS2 y graba las jugadas con HLAE, que instala solo. Los clips de stream no lo necesitan.",
  },
  {
    q: "¿Qué demos puedo usar?",
    a: "Demos de CS2 en .dem, .dem.zst (FACEIT) y .dem.gz, partidas de Matchmaking con su código para compartir y partidos pro de HLTV.",
  },
  {
    q: "¿Se sube algo a internet?",
    a: "No. Las demos, las grabaciones y los vídeos se quedan en tu PC. Tú decides dónde publicar.",
  },
  {
    q: "¿Puedo hacer vídeos de partidos pro?",
    a: "Sí. Abres un partido pro y grabas el POV del jugador que quieras, en Short o en partido entero.",
  },
  {
    q: "¿Qué PC necesito?",
    a: "El mismo con el que juegas a CS2, con Windows 10 u 11 de 64 bits. Con gráfica dedicada la grabación sale mejor.",
  },
] as const;

export const REQUIREMENTS = [
  "Windows 10 u 11 de 64 bits",
  "Counter-Strike 2 con Steam, para grabar vídeo",
  "HLAE oficial: Studio lo instala solo",
  "Gráfica dedicada para la mejor calidad",
  "Alrededor de 1 GB libre",
] as const;

export const NAV = [
  { href: "#shorts", label: "Shorts" },
  { href: "#demo-completa", label: "Demo completa" },
  { href: "#streams", label: "Streams" },
  { href: "#preguntas", label: "Preguntas" },
] as const;
