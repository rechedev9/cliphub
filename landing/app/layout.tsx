import type { Metadata } from "next";
import { Chakra_Petch, Share_Tech_Mono } from "next/font/google";
import "./globals.css";

// NEON HUD type system: Chakra Petch for display/sans, Share Tech Mono for
// every number/label/eyebrow. Same next/font/google wiring as web/app/layout.tsx.
const chakraPetch = Chakra_Petch({
  variable: "--font-chakra-petch",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

const shareTechMono = Share_Tech_Mono({
  variable: "--font-share-tech-mono",
  subsets: ["latin"],
  weight: "400",
  display: "swap",
});

const SOCIAL_TITLE = "Your CS2 frags, ready to post | TickCut";
const SITE_DESCRIPTION =
  "Turn CS2 demos and stream moments into polished vertical Shorts with local capture and ready-to-post edits.";

export const metadata: Metadata = {
  metadataBase: new URL("https://tickcut.gravityroom.app"),
  title: SOCIAL_TITLE,
  description: SITE_DESCRIPTION,
  applicationName: "TickCut Studio",
  keywords: [
    "CS2",
    "Counter-Strike 2",
    "demo to video",
    "vertical Shorts",
    "HLAE",
    "highlights",
    "frag movie",
  ],
  openGraph: {
    type: "website",
    siteName: "TickCut Studio",
    title: SOCIAL_TITLE,
    description: SITE_DESCRIPTION,
    url: "/",
    // Query busts Discord/Slack OG caches after the TickCut rebrand.
    images: [
      {
        url: "/opengraph-image?v=tickcut-1",
        width: 1200,
        height: 630,
        alt: "TickCut — Your best CS2 frags, ready to post",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: SOCIAL_TITLE,
    description: SITE_DESCRIPTION,
    images: ["/opengraph-image?v=tickcut-1"],
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body
        className={`${chakraPetch.variable} ${shareTechMono.variable} bg-[#050812] text-[#f2fbff] antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
