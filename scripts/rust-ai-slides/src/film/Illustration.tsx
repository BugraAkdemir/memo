import React from "react";
import {
  AbsoluteFill,
  Img,
  interpolate,
  staticFile,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import { C, Heading, Pill, Pop, clamp } from "./shared";
import { Python, Rust } from "../visuals";
type Props = { art: string; part: number; role: string; highlight: string };
export function Illustration({ art, part, role, highlight }: Props) {
  const f = useCurrentFrame(),
    { fps, durationInFrames } = useVideoConfig();
  const p = f / Math.max(1, durationInFrames);
  const zoom = 1.03 + Math.min(f / (18 * fps), 1) * 0.06;
  const config: Record<
    string,
    {
      title: string;
      labels: string[];
      point: [number, number];
      position: string;
    }
  > = {
    restaurant: {
      title: "Sipariş başka.\nMutfak başka.",
      labels: ["Koordinasyon", "Ağır hesaplama"],
      point: [1250, 600],
      position: part % 2 ? "70% 50%" : "35% 50%",
    },
    traffic: {
      title: "Motoru değiştirdin.\nKöprü hâlâ dar.",
      labels: ["Hızlı araba", "Gerçek darboğaz"],
      point: [1170, 545],
      position: "50% 50%",
    },
    workshop: {
      title: "Bütün sistemi\nsökmek şart mı?",
      labels: ["Çalışan sistem", "Değişen tek parça"],
      point: [1160, 550],
      position: "55% 50%",
    },
    bridge: {
      title: "Birini seçmek\nzorunda değilsin.",
      labels: ["Python uygulaması", "Rust modülü"],
      point: [1145, 650],
      position: "50% 50%",
    },
  };
  const c = config[art];
  const phase = Math.floor(f / (4.2 * fps)) % 2;
  return (
    <AbsoluteFill style={{ background: C.bg, overflow: "hidden" }}>
      <Img
        src={staticFile(`art/${art}.jpg`)}
        style={{
          position: "absolute",
          inset: 0,
          width: "100%",
          height: "100%",
          objectFit: "cover",
          objectPosition: c.position,
          scale: zoom,
          translate: `${Math.sin(f / (9 * fps)) * -14}px ${Math.cos(f / (12 * fps)) * 8}px`,
        }}
      />
      <AbsoluteFill
        style={{
          background:
            "linear-gradient(90deg,#09110ff5 0%,#09110fbf 32%,#09110f10 68%),linear-gradient(0deg,#09110ff2,transparent 24%)",
        }}
      />
      <Pop
        delay={0.1}
        style={{ position: "absolute", left: 105, top: 165, width: 1050 }}
      >
        <Heading small>{c.title}</Heading>
      </Pop>
      <Pop
        delay={0.5}
        x={-60}
        style={{ position: "absolute", left: 110, top: 435 }}
      >
        <Pill color={phase ? C.orange : C.blue}>{c.labels[phase]}</Pill>
      </Pop>
      {f > fps && (
        <svg
          width="1920"
          height="1080"
          style={{ position: "absolute", inset: 0 }}
        >
          <circle
            cx={c.point[0]}
            cy={c.point[1]}
            r={65 + Math.sin(f / (0.75 * fps)) * 5}
            fill="none"
            stroke={C.yellow}
            strokeWidth="3"
            opacity=".75"
            strokeDasharray="10 9"
          />
          <path
            d={`M${c.point[0] - 155} ${c.point[1] - 140} Q${c.point[0] - 40} ${c.point[1] - 130} ${c.point[0] - 20} ${c.point[1] - 70}`}
            fill="none"
            stroke={C.yellow}
            strokeWidth="4"
            strokeDasharray="250"
            strokeDashoffset={interpolate(f, [fps, 2 * fps], [250, 0], clamp)}
          />
        </svg>
      )}
      {art === "restaurant" && (
        <div
          style={{
            position: "absolute",
            left: interpolate(f, [2 * fps, 5 * fps], [780, 1340], clamp),
            top: interpolate(f, [2 * fps, 5 * fps], [680, 610], clamp),
            rotate: `${interpolate(f, [2 * fps, 5 * fps], [-12, 9], clamp)}deg`,
            opacity: interpolate(
              f,
              [1.8 * fps, 2.1 * fps, 5 * fps, 5.4 * fps],
              [0, 1, 1, 0],
              clamp,
            ),
            background: C.paper,
            width: 90,
            height: 120,
            padding: 15,
            borderRadius: 7,
            boxShadow: "0 15px 25px #0007",
          }}
        >
          {[0, 1, 2].map((i) => (
            <div
              key={i}
              style={{
                height: 4,
                marginTop: 15,
                width: 50 - i * 8,
                background: "#8c7960",
              }}
            />
          ))}
        </div>
      )}
      {art === "bridge" && (
        <div
          style={{
            position: "absolute",
            left: interpolate(f % (7 * fps), [0, 7 * fps], [680, 1350], clamp),
            top: 575 + Math.sin(f / (fps * 2)) * 25,
            opacity: 0.8,
            scale: 0.6,
          }}
        >
          <Pill color={C.yellow}>Veri</Pill>
        </div>
      )}
      {art === "workshop" && (
        <div
          style={{
            position: "absolute",
            left: 110,
            top: 545,
            display: "flex",
            alignItems: "center",
            gap: 22,
          }}
        >
          <Python size={65} />
          <span style={{ color: C.paper, fontSize: 36 }}>+</span>
          <Rust size={65} />
        </div>
      )}
      <div style={{ position: "absolute", bottom: 105, right: 105 }}>
        <Pop delay={1}>
          <Pill>
            {art === "traffic"
              ? "Önce sorunun yerini bul"
              : art === "bridge"
                ? "Bağlantı: PyO3"
                : art === "workshop"
                  ? "Önce küçük bir deney"
                  : "Aynı restoran, farklı görevler"}
          </Pill>
        </Pop>
      </div>
    </AbsoluteFill>
  );
}
