import { useId, type ReactNode } from 'react';

// Illustrative CS2 HUD for the picker, drawn on the same 1920x1080 frame as the
// broadcast previews so switching HUDs does not move the layout. The values are
// examples; the capture records the player's real HUD from the demo.
const PANEL = 'rgba(8, 10, 14, 0.62)';
const TEXT = '#f4f5f7';
const MUTED = 'rgba(244, 245, 247, 0.62)';
const CT = '#6fa8ff';
const T = '#f0b44c';

function Avatars({ x, color }: { x: number; color: string }): ReactNode {
  return <g>{[0, 1, 2, 3, 4].map((index) => <g key={index} transform={`translate(${x + index * 58} 14)`}>
    <rect width="50" height="50" rx="4" fill={PANEL} stroke={color} strokeOpacity=".55" strokeWidth="2" />
    <circle cx="25" cy="20" r="9" fill={MUTED} />
    <path d="M9 44c3-10 10-14 16-14s13 4 16 14z" fill={MUTED} />
    <rect y="54" width="50" height="5" rx="2" fill={index === 3 ? 'rgba(244,245,247,.18)' : color} />
  </g>)}</g>;
}

function Killfeed({ y, own }: { y: number; own?: boolean }): ReactNode {
  return <g transform={`translate(1480 ${y})`}>
    <rect width="420" height="40" rx="4" fill={PANEL} stroke={own ? '#e5484d' : 'none'} strokeWidth="3" />
    <rect x="16" y="15" width="104" height="10" rx="5" fill={own ? CT : T} />
    <path d="M150 14h78l10 6h22v8h-58l-8 6h-12l4-6h-36z" fill={TEXT} />
    <rect x="284" y="15" width="118" height="10" rx="5" fill={own ? T : CT} />
  </g>;
}

export function NativeHudArt(): ReactNode {
  // The preview renders twice (card and enlarged dialog); keep gradient ids unique.
  const id = `native-hud${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  return <svg viewBox="0 0 1920 1080" className="absolute inset-0 size-full" role="img" aria-label="Vista previa ilustrativa del HUD original de CS2">
    <g fontFamily="var(--font-display), system-ui, sans-serif" fill={TEXT}>
      {/* Radar and money */}
      <g transform="translate(24 24)">
        <rect width="300" height="300" rx="10" fill={PANEL} />
        <path d="M58 70h96v48h62v92h-70v40H70v-86H42z" fill="none" stroke={MUTED} strokeWidth="6" strokeLinejoin="round" />
        <path d="M154 118v-56h82v96" fill="none" stroke={MUTED} strokeWidth="6" strokeLinejoin="round" />
        <circle cx="206" cy="92" r="8" fill={CT} /><circle cx="96" cy="222" r="8" fill={CT} /><circle cx="176" cy="196" r="8" fill={CT} />
        <path d="M120 150l18 42-18-10-18 10z" fill={TEXT} />
      </g>
      <text x="36" y="372" fontSize="34" fontWeight="600" fill="#8ee07a">$ 4 750</text>

      {/* Team counter */}
      <Avatars x={596} color={CT} />
      <rect x="892" y="14" width="64" height="50" rx="4" fill={PANEL} /><text x="924" y="52" fontSize="34" fontWeight="700" textAnchor="middle" fill={CT}>7</text>
      <rect x="1000" y="14" width="64" height="50" rx="4" fill={PANEL} /><text x="1032" y="52" fontSize="34" fontWeight="700" textAnchor="middle" fill={T}>5</text>
      <rect x="906" y="70" width="144" height="40" rx="4" fill={PANEL} /><text x="978" y="100" fontSize="28" fontWeight="600" textAnchor="middle">1:42</text>
      <Avatars x={1074} color={T} />

      {/* Killfeed */}
      <Killfeed y={96} own />
      <Killfeed y={144} />

      {/* Player crosshair */}
      <g stroke="#4cff6a" strokeWidth="4" strokeLinecap="square">
        <path d="M960 522v-16M960 558v16M942 540h-16M978 540h16" />
      </g>

      {/* Health and armor */}
      <rect x="0" y="990" width="520" height="90" fill={`url(#${id}-left)`} />
      <path d="M44 1014h16v16h16v16H60v16H44v-16H28v-16h16z" fill={TEXT} />
      <text x="92" y="1060" fontSize="54" fontWeight="700">100</text>
      <path d="M240 1012l26 8v18c0 16-11 26-26 32-15-6-26-16-26-32v-18z" fill={TEXT} />
      <text x="282" y="1060" fontSize="54" fontWeight="700">100</text>

      {/* Weapon and ammunition */}
      <rect x="1400" y="990" width="520" height="90" fill={`url(#${id}-right)`} />
      <path d="M1506 1030h96l14 8h34v12h-84l-10 10h-22l6-10h-34z" fill={MUTED} />
      <text x="1768" y="1060" fontSize="54" fontWeight="700" textAnchor="end">30</text>
      <text x="1776" y="1060" fontSize="30" fill={MUTED}>/ 90</text>
    </g>
    <defs>
      <linearGradient id={`${id}-left`} x1="0" x2="1"><stop offset="0" stopColor="#000" stopOpacity=".55" /><stop offset="1" stopColor="#000" stopOpacity="0" /></linearGradient>
      <linearGradient id={`${id}-right`} x1="1" x2="0"><stop offset="0" stopColor="#000" stopOpacity=".55" /><stop offset="1" stopColor="#000" stopOpacity="0" /></linearGradient>
    </defs>
  </svg>;
}
