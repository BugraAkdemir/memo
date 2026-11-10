import React from "react";
import { createRoot } from "react-dom/client";
import { Player } from "@remotion/player";
import { Movie } from "./film/Movie";
import timeline from "./film-timeline.json";
import "./style.css";
const mins = Math.floor(timeline.totalSeconds / 60),
  secs = Math.floor(timeline.totalSeconds % 60)
    .toString()
    .padStart(2, "0");
createRoot(document.getElementById("root")!).render(
  <div
    style={{
      minHeight: "100vh",
      background: "#080f0d",
      padding: 30,
      display: "flex",
      flexDirection: "column",
      justifyContent: "center",
      gap: 20,
    }}
  >
    <header
      style={{
        display: "flex",
        justifyContent: "space-between",
        color: "#e7e9dc",
        fontWeight: 800,
      }}
    >
      <span>PYTHON × RUST · Seslendirmeli film</span>
      <span>
        {mins}:{secs} · 1080p / 30 fps
      </span>
    </header>
    <Player
      component={Movie}
      durationInFrames={timeline.totalFrames}
      fps={30}
      compositionWidth={1920}
      compositionHeight={1080}
      controls
      clickToPlay
      spaceKeyToPlayOrPause
      style={{ width: "100%", maxHeight: "85vh", aspectRatio: "16/9" }}
    />
    <div style={{ color: "#9daf9b", fontSize: 14 }}>
      ElevenLabs anlatım · Gerçek ses sürelerine göre kurgu · Tam ekran için
      oynatıcı düğmesini kullan.
    </div>
  </div>,
);
