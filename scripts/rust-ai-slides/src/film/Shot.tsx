import React from "react";
import {
  AbsoluteFill,
  interpolate,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { Concept } from "./Concept";
import { Illustration } from "./Illustration";
import { Pipeline } from "./Pipeline";
import { C, Pill, Pop, clamp } from "./shared";
import { Mascot } from "../visuals";
import { slides, chapterNames } from "../data";
import timeline from "../film-timeline.json";
export type Beat = {
  id: string;
  scene: number;
  part: number;
  role: string;
  tr: string;
  highlight: string;
  mode: string;
  from: number;
  duration: number;
  lead: number;
  voiceSec: number;
  chapterStart: boolean;
};
export function Shot({ beat }: { beat: Beat }) {
  const f = useCurrentFrame(),
    { fps, durationInFrames } = useVideoConfig(),
    s = slides[beat.scene];
  const macro = Math.floor(f / (7 * fps));
  const camera = interpolate(
    f,
    [0, Math.max(fps, durationInFrames)],
    [1, 1.025],
    clamp,
  );
  const isArt = s.kind === "art";
  const role =
    beat.role === "classic" || beat.role === "pixel" ? beat.role : null;
  const speaking =
    role && f >= beat.lead && f < beat.lead + beat.voiceSec * fps;
  const isPipeline = [
    "document",
    "engine",
    "stack",
    "tokens",
    "inside",
    "where",
    "module",
  ].includes(s.kind);
  return (
    <AbsoluteFill style={{ background: C.bg, overflow: "hidden" }}>
      <div
        style={{
          position: "absolute",
          inset: 0,
          scale: camera,
          transformOrigin: "center",
        }}
      >
        {isArt ? (
          <Illustration
            art={s.art!}
            part={beat.part + macro}
            role={beat.role}
            highlight={beat.highlight}
          />
        ) : isPipeline ? (
          <Pipeline kind={s.kind} part={beat.part + macro} />
        ) : (
          <Concept kind={s.kind} part={beat.part + macro} mode={beat.mode} />
        )}
      </div>
      <div
        style={{
          position: "absolute",
          top: 50,
          left: 105,
          right: 105,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          fontSize: 21,
          letterSpacing: 2,
          color: "#cfdbce",
          fontFamily: "Manrope,sans-serif",
          fontWeight: 800,
        }}
      >
        <span>
          {String(s.chapter).padStart(2, "0")} /{" "}
          {chapterNames[s.chapter - 1].toLocaleUpperCase("tr")}
        </span>
        <span style={{ fontSize: 19, color: "#a4b5a9" }}>PYTHON × RUST</span>
      </div>
      {s.source && (
        <div
          style={{
            position: "absolute",
            bottom: 39,
            right: 105,
            fontFamily: "Manrope,sans-serif",
            fontSize: 18,
            color: "#9caf9e",
          }}
        >
          {s.source}
        </div>
      )}
      {role && (
        <div
          style={{
            position: "absolute",
            bottom: 90,
            left: 70,
            display: "flex",
            alignItems: "flex-end",
            gap: 10,
          }}
        >
          <div style={{ translate: `0 ${Math.sin(f / (fps * 0.6)) * 3}px` }}>
            <Mascot
              skin={role}
              pose={speaking ? "talk" : "point"}
              frame={f}
              size={230}
            />
          </div>
          <Pop delay={0.5} style={{ marginBottom: 22 }}>
            <Pill color={role === "pixel" ? C.blue : C.yellow}>
              {role === "pixel" ? "Mavi Memo" : "Memo"} · örnek zamanı
            </Pill>
          </Pop>
        </div>
      )}
      {!role && ["intro", "s06-p1", "s11-p1", "s22-p1"].includes(beat.id) && (
        <div
          style={{
            position: "absolute",
            right: 90,
            bottom: 100,
            display: "flex",
            gap: 5,
          }}
        >
          <Mascot
            skin="classic"
            pose={s.kind === "ending" ? "wave" : "think"}
            frame={f}
            size={145}
          />
          <Mascot
            skin="pixel"
            pose={s.kind === "ending" ? "wave" : "point"}
            frame={f + 20}
            size={145}
          />
        </div>
      )}
      {beat.chapterStart && (
        <div
          style={{
            position: "absolute",
            inset: 0,
            background: C.yellow,
            translate: `${interpolate(f, [0, 0.4 * fps], [-1920, 1920], clamp)}px 0`,
            opacity: f < 0.4 * fps ? 0.92 : 0,
            pointerEvents: "none",
          }}
        />
      )}
      <div
        style={{
          position: "absolute",
          left: 0,
          right: 0,
          bottom: 0,
          height: 3,
          background: "#273c2d",
        }}
      >
        <div
          style={{
            height: "100%",
            background: `linear-gradient(90deg,${C.blue},${C.yellow},${C.orange})`,
            width: `${((beat.from + f) / timeline.totalFrames) * 100}%`,
          }}
        />
      </div>
    </AbsoluteFill>
  );
}
