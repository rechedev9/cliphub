import type { Metadata, Viewport } from "next";
import { IBM_Plex_Mono, Saira, Saira_Condensed } from "next/font/google";
import "./globals.css";

const saira = Saira({ variable: "--font-saira", subsets: ["latin"], weight: ["400", "500", "600", "700"], display: "swap" });
const sairaCondensed = Saira_Condensed({
  variable: "--font-saira-condensed",
  subsets: ["latin"],
  weight: ["500", "700", "800"],
  display: "swap",
});
const plexMono = IBM_Plex_Mono({ variable: "--font-plex-mono", subsets: ["latin"], weight: ["400", "500", "600"], display: "swap" });

const TITLE = "Análisis de demos de CS2 gratis para equipos | ClipHub";
const DESCRIPTION =
  "Lee al rival antes de que juegue: replay 2D, scouting, bombsites, playbook, veto y comms de voz. Tus demos no salen de tu ordenador. Gratis, en el navegador o en Windows.";
const OG_IMAGE = "/opengraph-image?v=analytics-1";

export const metadata: Metadata = {
  metadataBase: new URL("https://cliphub.rechedev.cloud"),
  title: TITLE,
  description: DESCRIPTION,
  applicationName: "ClipHub",
  keywords: ["CS2", "Counter-Strike 2", "análisis de demos", "scouting", "anti-strat", "replay 2D", "comms", "coach", "IGL"],
  openGraph: {
    type: "website",
    locale: "es_ES",
    siteName: "ClipHub",
    title: TITLE,
    description: DESCRIPTION,
    url: "/",
    images: [{ url: OG_IMAGE, width: 1200, height: 630, alt: "El replay 2D de ClipHub con el mapa de calor de un equipo en Cache" }],
  },
  twitter: { card: "summary_large_image", title: TITLE, description: DESCRIPTION, images: [OG_IMAGE] },
};

export const viewport: Viewport = {
  themeColor: "#000000",
  colorScheme: "dark",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="es" className={`${saira.variable} ${sairaCondensed.variable} ${plexMono.variable}`}>
      <body className="font-sans antialiased">{children}</body>
    </html>
  );
}
