import React from "react";
import { interpolate, useCurrentFrame, useVideoConfig } from "remotion";
import { C, clamp } from "./shared";
export function SocialLogo({
  platform,
  size = 25,
}: {
  platform: "youtube" | "github" | "instagram";
  size?: number;
}) {
  if (platform === "youtube")
    return (
      <svg width={size} height={size} viewBox="0 0 32 32" aria-label="YouTube">
        <rect x="1" y="5" width="30" height="22" rx="7" fill="#ff3b38" />
        <path d="M13 10l10 6-10 6z" fill="#fff" />
      </svg>
    );
  if (platform === "instagram")
    return (
      <svg
        width={size}
        height={size}
        viewBox="0 0 32 32"
        fill="none"
        aria-label="Instagram"
      >
        <defs>
          <linearGradient id="ig" x1="0" y1="32" x2="32" y2="0">
            <stop stopColor="#ffd26e" />
            <stop offset=".5" stopColor="#f85a8e" />
            <stop offset="1" stopColor="#a882ff" />
          </linearGradient>
        </defs>
        <rect
          x="3"
          y="3"
          width="26"
          height="26"
          rx="8"
          stroke="url(#ig)"
          strokeWidth="3"
        />
        <circle cx="16" cy="16" r="6" stroke="url(#ig)" strokeWidth="3" />
        <circle cx="24" cy="8" r="1.8" fill="#f58ab3" />
      </svg>
    );
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="#f4f0e6"
      aria-label="GitHub"
    >
      <path d="M12 .7a11.3 11.3 0 0 0-3.57 22c.56.1.77-.24.77-.54v-2.1c-3.15.68-3.82-1.34-3.82-1.34-.52-1.3-1.26-1.65-1.26-1.65-1.03-.7.08-.69.08-.69 1.14.08 1.74 1.17 1.74 1.17 1.02 1.75 2.66 1.24 3.31.95.1-.74.4-1.24.72-1.53-2.51-.29-5.15-1.26-5.15-5.59 0-1.24.44-2.25 1.17-3.04-.12-.29-.51-1.45.11-3.02 0 0 .95-.3 3.11 1.16a10.8 10.8 0 0 1 5.66 0c2.16-1.46 3.11-1.16 3.11-1.16.62 1.57.23 2.73.11 3.02.73.79 1.17 1.8 1.17 3.04 0 4.34-2.65 5.3-5.17 5.58.4.35.77 1.04.77 2.1v3.11c0 .3.2.65.78.54A11.3 11.3 0 0 0 12 .7z" />
    </svg>
  );
}
// A center crop from 1920×1080 to 9:16 retains x=656.25..1263.75.
// Both this 530px identity plate and the 548px subtitle area sit inside it.
export function Branding() {
  const f = useCurrentFrame(),
    { fps } = useVideoConfig();
  const rows = [
    { platform: "youtube" as const, name: "@BugraAkdemir" },
    { platform: "github" as const, name: "BugraAkdemir" },
    { platform: "instagram" as const, name: "@bugra.code" },
  ];
  return (
    <div
      data-branding="portrait-safe"
      style={{
        position: "absolute",
        left: 695,
        top: 14,
        width: 530,
        padding: "9px 22px",
        borderRadius: 16,
        background: "#07120ed9",
        border: "1px solid #80967b44",
        fontFamily: "Manrope,sans-serif",
        opacity: interpolate(f, [0, 0.6 * fps], [0, 1], clamp),
        boxShadow: "0 5px 22px #0003",
      }}
    >
      {rows.map((r) => (
        <div
          key={r.platform}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 13,
            height: 29,
            color: C.paper,
            fontSize: 23,
            fontWeight: 700,
            letterSpacing: 0.1,
          }}
        >
          <SocialLogo platform={r.platform} size={24} />
          <span>{r.name}</span>
        </div>
      ))}
    </div>
  );
}
