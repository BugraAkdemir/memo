import type { Caption } from "@remotion/captions";
import { interpolate, useCurrentFrame, useVideoConfig } from "remotion";
import captions from "../captions.json";
// The single source here is the exact master narration WAV used by Movie.
// offsetMs permits a standalone shot preview without changing source timing.
export function Captions({ offsetMs = 0 }: { offsetMs?: number }) {
  const f = useCurrentFrame(),
    { fps } = useVideoConfig(),
    ms = (f / fps) * 1000 + offsetMs;
  const all = captions as Caption[];
  let lo = 0,
    hi = all.length - 1,
    active: Caption | undefined;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1,
      c = all[mid];
    if (ms < c.startMs) hi = mid - 1;
    else if (ms >= c.endMs) lo = mid + 1;
    else {
      active = c;
      break;
    }
  }
  if (!active) return null;
  return (
    <div
      data-captions="portrait-safe"
      style={{
        position: "absolute",
        left: 686,
        width: 548,
        bottom: 51,
        display: "flex",
        justifyContent: "center",
        pointerEvents: "none",
      }}
    >
      <div
        style={{
          color: "#fff9ed",
          background: "#06100ef2",
          border: "1px solid #6b897455",
          boxShadow: "0 8px 25px #0005",
          padding: "13px 20px",
          borderRadius: 17,
          fontFamily: "Manrope,sans-serif",
          fontSize: active.text.length > 48 ? 32 : 37,
          fontWeight: 800,
          lineHeight: 1.2,
          textAlign: "center",
          letterSpacing: -0.55,
          whiteSpace: "pre-wrap",
          textWrap: "balance",
          opacity: interpolate(
            ms,
            [active.startMs, active.startMs + 70],
            [0, 1],
            { extrapolateLeft: "clamp", extrapolateRight: "clamp" },
          ),
        }}
      >
        {active.text.trim()}
      </div>
    </div>
  );
}
