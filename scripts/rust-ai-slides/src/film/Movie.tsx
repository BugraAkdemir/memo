import {
  AbsoluteFill,
  interpolate,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { Audio } from "@remotion/media";
import { AuthoredTimeline } from "./timeline-nodes";
import timeline from "../film-timeline.json";
import { Shot } from "./Shot";
import { Branding } from "./Branding";
import { Captions } from "./Captions";
import "../style.css";
export function Movie() {
  const f = useCurrentFrame(),
    { fps, durationInFrames } = useVideoConfig();
  return (
    <AbsoluteFill style={{ background: "#101719" }}>
      <AuthoredTimeline />
      <Branding />
      <Captions />
      <Audio
        src={staticFile("voice/film-narration.wav")}
        volume={1}
        premountFor={fps}
      />
      {timeline.beats
        .filter((b) => b.chapterStart)
        .map((b) => (
          <Audio
            key={b.id}
            src={staticFile("sfx/whoosh.wav")}
            from={b.from}
            volume={0.55}
            premountFor={fps}
          />
        ))}
      <AbsoluteFill
        style={{
          background: "#000",
          opacity: interpolate(
            f,
            [0, 15, durationInFrames - 25, durationInFrames - 1],
            [1, 0, 0, 1],
            { extrapolateLeft: "clamp", extrapolateRight: "clamp" },
          ),
          pointerEvents: "none",
        }}
      />
    </AbsoluteFill>
  );
}
export function ShotPreview({ beatIndex }: { beatIndex: number }) {
  return (
    <>
      <Shot
        beat={
          timeline.beats[
            Math.max(0, Math.min(timeline.beats.length - 1, beatIndex))
          ]
        }
      />
      <Branding />
      <Captions
        offsetMs={
          (timeline.beats[
            Math.max(0, Math.min(timeline.beats.length - 1, beatIndex))
          ].from /
            30) *
          1000
        }
      />
    </>
  );
}
