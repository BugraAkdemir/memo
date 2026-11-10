import React from "react";
import {
  AbsoluteFill,
  interpolate,
  useCurrentFrame,
  useVideoConfig,
} from "remotion";
import {
  Backdrop,
  C,
  Connector,
  Heading,
  Icon,
  MotionCard,
  Pill,
  Pop,
  clamp,
} from "./shared";
import { Python, Rust } from "../visuals";
type Props = { kind: string; part: number };
export function Pipeline({ kind, part }: Props) {
  const f = useCurrentFrame(),
    { fps } = useVideoConfig();
  const phase = Math.floor(f / (3.8 * fps));
  const isToken = kind === "tokens",
    isModule = kind === "module",
    isStack = kind === "stack" || kind === "engine";
  const titles: Record<string, string> = {
    document: "Bir PDF’nin yolculuğu",
    where: "Cevabı nerede bekliyorsun?",
    tokens: "Model kelimeleri nasıl görüyor?",
    module: "Uygulama kalsın. Bir parçayı dene.",
    stack: "Üstte Python. Altta hesaplama.",
    engine: "Direksiyon başka. Motor başka.",
    inside: "Python kullanırken içeride Rust olabilir.",
  };
  return (
    <Backdrop accent={isModule ? C.orange : C.blue}>
      <div style={{ position: "absolute", top: 130, left: 100, right: 100 }}>
        <Pop>
          <Heading small>{titles[kind]}</Heading>
        </Pop>
      </div>
      {isToken ? (
        <>
          <Pop
            delay={0.35}
            style={{ position: "absolute", left: 100, top: 310 }}
          >
            <Pill>“Kedim bugün çok mutlu.”</Pill>
          </Pop>
          <div
            style={{
              position: "absolute",
              top: 465,
              left: 120,
              right: 120,
              display: "flex",
              gap: 20,
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            {["Ked", "im", " bugün", " çok", " mutlu", "."].map((word, i) => (
              <Pop
                key={i}
                delay={0.65 + i * 0.12}
                x={i % 2 ? 60 : -60}
                y={50}
                style={{
                  border: `2px solid ${[C.blue, C.yellow, C.orange][i % 3]}`,
                  padding: "30px 33px",
                  borderRadius: 20,
                  background: "#203129",
                  rotate: `${Math.sin(f / (fps * 2) + i) * 1.5}deg`,
                }}
              >
                <strong style={{ fontSize: 49 }}>{word}</strong>
              </Pop>
            ))}
          </div>
          <Pop
            delay={2}
            style={{
              position: "absolute",
              left: 650,
              top: 670,
              display: "flex",
              alignItems: "center",
              gap: 30,
            }}
          >
            <Icon name="brain" color={C.orange} size={130} />
            <span style={{ fontSize: 38, fontWeight: 800 }}>
              Token dizisi → model
            </span>
          </Pop>
          <div
            style={{
              position: "absolute",
              bottom: 110,
              left: 110,
              fontSize: 22,
              color: C.muted,
            }}
          >
            Parçalar temsilidir; belirli bir modelin çıktısı değildir.
          </div>
        </>
      ) : isModule || kind === "inside" ? (
        <>
          <Pop
            delay={0.25}
            style={{
              position: "absolute",
              left: 150,
              top: 300,
              width: 1620,
              height: 520,
              border: "2px solid #68b8f777",
              borderRadius: 35,
              background: "#152b2933",
              padding: 40,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 25 }}>
              <Python size={85} />
              <strong style={{ fontSize: 42 }}>Python uygulaman</strong>
            </div>
            <div style={{ display: "flex", gap: 25, marginTop: 40 }}>
              {[
                "Arayüz",
                kind === "inside" ? "Tokenizers" : "Ölçülen bölüm",
                "Diğer işler",
              ].map((label, i) => (
                <Pop
                  delay={0.7 + i * 0.13}
                  key={label}
                  style={{
                    background: i === 1 ? "#472f21" : "#20372c",
                    border: `2px solid ${i === 1 ? C.orange : "#405c4a"}`,
                    borderRadius: 24,
                    padding: 32,
                    width: i === 1 ? 610 : 380,
                    height: 260,
                    display: "flex",
                    flexDirection: "column",
                    gap: 25,
                    alignItems: "center",
                    justifyContent: "center",
                    scale: i === 1 ? 1 + Math.sin(f / (fps * 3)) * 0.012 : 1,
                  }}
                >
                  {i === 1 ? (
                    <Rust size={100} />
                  ) : (
                    <Icon name={i === 0 ? "app" : "file"} size={90} />
                  )}
                  <strong style={{ fontSize: 32 }}>
                    {i === 1 && kind === "module" && phase > 0
                      ? "Rust modülü"
                      : label}
                  </strong>
                </Pop>
              ))}
            </div>
          </Pop>
          <Pop delay={2} style={{ position: "absolute", left: 150, top: 875 }}>
            <Pill color={C.orange}>
              {kind === "inside"
                ? "Bütün uygulamayı taşımak gerekmiyor."
                : "Küçük deney → ölçüm → karar"}
            </Pill>
          </Pop>
        </>
      ) : isStack ? (
        <>
          <div
            style={{
              position: "absolute",
              left: 160,
              top: 335,
              display: "flex",
              gap: 35,
              alignItems: "center",
            }}
          >
            <Pop>
              <div
                style={{
                  width: 420,
                  height: 390,
                  borderRadius: 30,
                  border: "2px solid #68b8f777",
                  background: "#1c302a",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 25,
                }}
              >
                <Python size={150} />
                <strong style={{ fontSize: 42 }}>Python / PyTorch</strong>
                <span style={{ fontSize: 28, color: C.muted }}>
                  İşi tarif et
                </span>
              </div>
            </Pop>
            <Pop delay={0.5}>
              <Connector
                progress={interpolate(f, [0.4 * fps, 1.4 * fps], [0, 1], clamp)}
              />
            </Pop>
            <Pop delay={0.8}>
              <div
                style={{
                  width: 420,
                  height: 390,
                  borderRadius: 30,
                  border: "2px solid #f7d36d88",
                  background: "#302f20",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 25,
                }}
              >
                <Icon name="tool" size={150} color={C.yellow} />
                <strong style={{ fontSize: 42 }}>C++ altyapısı</strong>
                <span style={{ fontSize: 28, color: C.muted }}>
                  Hesaplamayı yürüt
                </span>
              </div>
            </Pop>
            <Pop delay={1.1}>
              <Connector
                progress={interpolate(f, [fps, 2 * fps], [0, 1], clamp)}
              />
            </Pop>
            <Pop delay={1.4}>
              <div
                style={{
                  width: 365,
                  height: 390,
                  borderRadius: 30,
                  border: "2px solid #f3976888",
                  background: "#35271f",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 25,
                }}
              >
                <div style={{ rotate: `${Math.sin(f / (fps * 2)) * 4}deg` }}>
                  <Icon name="chip" size={150} color={C.orange} />
                </div>
                <strong style={{ fontSize: 42 }}>CPU / GPU</strong>
                <span style={{ fontSize: 28, color: C.muted }}>
                  Donanımda çalıştır
                </span>
              </div>
            </Pop>
          </div>
          <Pop
            delay={2.2}
            style={{ position: "absolute", left: 160, top: 820 }}
          >
            <Pill>Üst katmanı değiştirmek her hesabı değiştirmez.</Pill>
          </Pop>
        </>
      ) : (
        <>
          <div
            style={{
              position: "absolute",
              left: 130,
              right: 130,
              top: 350,
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            {[
              { name: "file", title: "Dosya", color: C.blue },
              { name: "box", title: "Hazırlık", color: C.yellow },
              {
                name: kind === "where" ? "cloud" : "brain",
                title: kind === "where" ? "Ağ" : "Model",
                color: C.orange,
              },
              {
                name: kind === "where" ? "chip" : "app",
                title: kind === "where" ? "Model" : "Yanıt",
                color: C.green,
              },
            ].map((x, i) => (
              <React.Fragment key={x.title}>
                {i > 0 && (
                  <Pop delay={i * 0.22}>
                    <Connector
                      progress={interpolate(
                        f,
                        [i * 0.22 * fps, (i * 0.22 + 0.6) * fps],
                        [0, 1],
                        clamp,
                      )}
                    />
                  </Pop>
                )}
                <Pop
                  delay={i * 0.22}
                  style={{
                    width: 290,
                    height: 340,
                    background: phase % 4 === i ? "#2e4436" : "#192b23",
                    border: `2px solid ${phase % 4 === i ? x.color : "#3d5144"}`,
                    borderRadius: 27,
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    justifyContent: "center",
                    gap: 30,
                    translate: `0 ${Math.sin(f / (fps * 1.8) + i) * 5}px`,
                  }}
                >
                  <Icon name={x.name} color={x.color} size={130} />
                  <strong style={{ fontSize: 39 }}>{x.title}</strong>
                </Pop>
              </React.Fragment>
            ))}
          </div>
          <div
            style={{
              position: "absolute",
              left: 190 + ((f / (fps * 5)) % 1) * 1450,
              top: 730,
              width: 16,
              height: 16,
              background: C.yellow,
              borderRadius: 10,
              boxShadow: `0 0 25px ${C.yellow}`,
            }}
          />
          <Pop
            delay={1.5}
            style={{ position: "absolute", left: 130, top: 815 }}
          >
            <Pill>
              {kind === "where"
                ? "Dosya mı? Tekrar işlem mi? Ağ mı? Model mi?"
                : "Her parçanın ihtiyacı farklı."}
            </Pill>
          </Pop>
        </>
      )}
    </Backdrop>
  );
}
