import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import path from "node:path";

export const alt = "El replay 2D de ClipHub con el mapa de calor de un equipo en Cache";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const BRAND = "#22e07c";

export default async function OpengraphImage() {
  const shot = await readFile(path.join(process.cwd(), "public", "shots", "og-base.jpg"));
  const mark = await readFile(path.join(process.cwd(), "public", "brand", "cliphub-mark.svg"), "utf8");
  const shotSource = `data:image/jpeg;base64,${shot.toString("base64")}`;
  const markSource = `data:image/svg+xml;base64,${Buffer.from(mark).toString("base64")}`;

  return new ImageResponse(
    <div style={{ width: "100%", height: "100%", display: "flex", backgroundColor: "#080b0a", color: "#e8eeeb" }}>
      <div style={{ width: 560, display: "flex", flexDirection: "column", justifyContent: "space-between", padding: 56 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <img alt="" src={markSource} width={40} height={40} style={{ width: 40, height: 40 }} />
          <div style={{ display: "flex", fontSize: 30, fontWeight: 700 }}>ClipHub</div>
        </div>
        <div style={{ display: "flex", flexDirection: "column" }}>
          <div style={{ display: "flex", fontSize: 20, color: BRAND, letterSpacing: 3 }}>ANÁLISIS DE CS2 PARA EQUIPOS</div>
          <div style={{ display: "flex", marginTop: 18, fontSize: 52, fontWeight: 700, lineHeight: 1.08 }}>
            Prepara cada partido con todo lo que hay en la demo.
          </div>
          <div style={{ display: "flex", marginTop: 20, fontSize: 24, color: "#aab4b0" }}>
            En el navegador o en Windows. Gratis.
          </div>
        </div>
      </div>
      <div style={{ flex: 1, display: "flex", alignItems: "center", paddingRight: 0 }}>
        <img
          alt=""
          src={shotSource}
          width={800}
          height={600}
          style={{ width: 800, height: 600, borderRadius: 12, border: "1px solid #2b3532", objectFit: "cover" }}
        />
      </div>
    </div>,
    { ...size },
  );
}
