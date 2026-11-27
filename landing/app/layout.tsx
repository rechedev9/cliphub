import type { Metadata, Viewport } from "next";
import { IBM_Plex_Mono, Saira, Saira_Condensed } from "next/font/google";
import "./globals.css";

const saira = Saira({ variable: "--font-saira", subsets: ["latin"], weight: ["400", "500", "600", "700"], display: "swap" });
const sairaCondensed = Saira_Condensed({
  variable: "--font-saira-condensed",
  subsets: ["latin"],
  weight: ["500", "700", "800", "900"],
  display: "swap",
});
const plexMono = IBM_Plex_Mono({ variable: "--font-plex-mono", subsets: ["latin"], weight: ["400", "500", "600"], display: "swap" });

const TITLE = "Clips y Shorts de CS2 desde tus demos | ClipHub Studio";
const DESCRIPTION =
  "Studio graba tus jugadas dentro de CS2 y monta Shorts verticales, el partido entero con HUD de retransmisión y clips de tu stream. Empieza gratis en Windows, o deja que editemos por ti.";

export const metadata: Metadata = {
  metadataBase: new URL("https://cliphub.rechedev.cloud"),
  title: TITLE,
  description: DESCRIPTION,
  applicationName: "ClipHub Studio",
  keywords: ["CS2", "Counter-Strike 2", "clips", "Shorts", "highlights", "demo", "POV", "creadores", "stream"],
  openGraph: {
    type: "website",
    locale: "es_ES",
    siteName: "ClipHub Studio",
    title: TITLE,
    description: DESCRIPTION,
    url: "/",
  },
  twitter: { card: "summary_large_image", title: TITLE, description: DESCRIPTION },
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
