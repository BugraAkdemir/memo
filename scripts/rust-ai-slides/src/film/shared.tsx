import React from "react";
import {
  AbsoluteFill,
  Easing,
  interpolate,
  spring,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { C, Icon } from "../visuals";
export { C, Icon };
export const clamp = {
  extrapolateLeft: "clamp",
  extrapolateRight: "clamp",
} as const;
export function Pop({
  children,
  delay = 0,
  x = 0,
  y = 55,
  style = {},
}: {
  children: React.ReactNode;
  delay?: number;
  x?: number;
  y?: number;
  style?: React.CSSProperties;
}) {
  const f = useCurrentFrame(),
    { fps } = useVideoConfig();
  const p = spring({
    frame: f - delay * fps,
    fps,
    config: { damping: 22, stiffness: 120 },
  });
  return (
    <div
      style={{
        opacity: interpolate(
          f,
          [delay * fps, delay * fps + 0.25 * fps],
          [0, 1],
          clamp,
        ),
        translate: `${x * (1 - p)}px ${y * (1 - p)}px`,
        scale: 0.94 + 0.06 * p,
        ...style,
      }}
    >
      {children}
    </div>
  );
}
export function Pill({
  children,
  color = C.yellow,
}: {
  children: React.ReactNode;
  color?: string;
}) {
  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 16,
        border: `1px solid ${color}66`,
        background: "#15221fe8",
        color,
        padding: "15px 24px",
        borderRadius: 14,
        fontSize: 29,
        fontWeight: 800,
        whiteSpace: "nowrap",
      }}
    >
      {children}
    </div>
  );
}
export function Heading({
  children,
  small = false,
  color = C.paper,
}: {
  children: React.ReactNode;
  small?: boolean;
  color?: string;
}) {
  return (
    <h1
      style={{
        fontFamily: "Bricolage,Manrope,sans-serif",
        fontWeight: 800,
        fontSize: small ? 82 : 112,
        lineHeight: 1.02,
        letterSpacing: -4,
        color,
        margin: 0,
        whiteSpace: "pre-line",
      }}
    >
      {children}
    </h1>
  );
}
export function MotionCard({
  icon,
  title,
  label,
  color = C.blue,
  active = true,
  index = 0,
}: {
  icon: string;
  title: string;
  label?: string;
  color?: string;
  active?: boolean;
  index?: number;
}) {
  const f = useCurrentFrame(),
    { fps } = useVideoConfig();
  return (
    <Pop
      delay={index * 0.09}
      y={75}
      style={{
        position: "relative",
        width: 480,
        height: 395,
        padding: "44px 38px",
        borderRadius: 28,
        background: active ? "#26372f" : "#18231f",
        border: `2px solid ${active ? color + "aa" : "#35433d"}`,
        boxShadow: "0 24px 45px #0004",
        rotate: `${interpolate(f, [0, fps], [(index % 2 ? 1 : -1) * 3, 0], clamp)}deg`,
        opacity: active ? 1 : 0.45,
      }}
    >
      <div
        style={{
          position: "absolute",
          top: 0,
          left: 40,
          right: 40,
          height: 4,
          background: color,
          borderRadius: 5,
        }}
      />
      <Icon name={icon} size={118} color={color} />
      <h2
        style={{
          fontSize: 43,
          lineHeight: 1.1,
          letterSpacing: -1.4,
          margin: "28px 0 15px",
        }}
      >
        {title}
      </h2>
      {label && <div style={{ fontSize: 27, color: "#afc4b7" }}>{label}</div>}
    </Pop>
  );
}
export function Backdrop({
  children,
  accent = C.blue,
}: {
  children: React.ReactNode;
  accent?: string;
}) {
  const f = useCurrentFrame();
  return (
    <AbsoluteFill
      style={{
        background: "#101719",
        color: C.paper,
        fontFamily: "Manrope,sans-serif",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          position: "absolute",
          width: 1200,
          height: 1200,
          right: -450,
          top: -500,
          background: `radial-gradient(circle,${accent}12,transparent 66%)`,
        }}
      />
      <svg
        width="1920"
        height="1080"
        style={{ position: "absolute", inset: 0, opacity: 0.12 }}
      >
        <defs>
          <pattern
            id="grid"
            width="80"
            height="80"
            patternUnits="userSpaceOnUse"
          >
            <path
              d="M80 0H0V80"
              fill="none"
              stroke="#8da697"
              strokeWidth=".5"
            />
          </pattern>
        </defs>
        <rect width="1920" height="1080" fill="url(#grid)" />
      </svg>
      {Array.from({ length: 6 }, (_, i) => (
        <div
          key={i}
          style={{
            position: "absolute",
            width: 4,
            height: 4,
            borderRadius: 4,
            background: accent,
            opacity: 0.15,
            left: 180 + i * 310,
            top: 200 + ((i * 137) % 700) + Math.sin(f / 90 + i) * 14,
          }}
        />
      ))}
      {children}
    </AbsoluteFill>
  );
}
export function Connector({
  progress = 1,
  color = C.yellow,
}: {
  progress?: number;
  color?: string;
}) {
  return (
    <svg width="135" height="80" viewBox="0 0 135 80">
      <path
        d="M5 40h110m-23-22 24 22-24 22"
        fill="none"
        stroke={color}
        strokeWidth="6"
        strokeLinecap="round"
        strokeDasharray="175"
        strokeDashoffset={175 * (1 - progress)}
      />
    </svg>
  );
}
