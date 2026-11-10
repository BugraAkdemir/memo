import { AbsoluteFill } from "remotion";
import { SocialLogo } from "./Branding";
import { Python, Rust } from "../visuals";
import "../style.css";
const hooks = [
  ["PYTHON GÖRÜNÜYOR.", "ALTINDA NE VAR?"],
  ["PYTHON YAZIYORSUN.", "İÇİNDE RUST VAR."],
  ["DAHA HIZLI DİL,", "DAHA HIZLI CEVAP?"],
  ["RUST ÖĞRENMELİ", "MİSİN?"],
];
const labels = [
  "GÖRÜNEN DİL ≠ BÜTÜN SİSTEM",
  "AYNI UYGULAMA, FARKLI GÖREVLER",
  "ÖNCE DARBOĞAZI BUL",
  "KARAR, YAPTIĞIN İŞE BAĞLI",
];
export function ReelBackground({ index = 0 }: { index: number }) {
  const color = ["#60b9f2", "#ec9864", "#efd36c", "#60b9f2"][index];
  return (
    <AbsoluteFill
      style={{
        background: "#101719",
        color: "#f4f0e6",
        fontFamily: "Manrope,sans-serif",
      }}
    >
      <AbsoluteFill
        style={{
          background: `radial-gradient(circle at 100% 30%,${color}16,transparent 48%)`,
        }}
      />
      <div
        style={{
          position: "absolute",
          left: 76,
          top: 140,
          display: "flex",
          alignItems: "center",
          gap: 24,
          fontSize: 27,
          letterSpacing: 3,
          color: "#b4c7b9",
        }}
      >
        <Python size={48} />
        <span>PYTHON × RUST</span>
        <Rust size={48} />
      </div>
      <div
        style={{
          position: "absolute",
          left: 76,
          top: 245,
          right: 76,
          fontFamily: "Bricolage,Manrope,sans-serif",
          fontSize: index === 3 ? 88 : 76,
          fontWeight: 800,
          lineHeight: 1.03,
          letterSpacing: -3,
        }}
      >
        {hooks[index].map((x, i) => (
          <div
            key={i}
            style={{ whiteSpace: "nowrap", color: i ? color : "#f4f0e6" }}
          >
            {x}
          </div>
        ))}
      </div>
      <div
        style={{
          position: "absolute",
          left: 76,
          top: 480,
          color: "#a8bdaf",
          fontSize: 24,
          letterSpacing: 1.5,
        }}
      >
        {labels[index]}
      </div>
      <div
        style={{
          position: "absolute",
          left: 28,
          top: 574,
          width: 1024,
          height: 449,
          border: "1px solid #6d867b55",
          borderRadius: 18,
          boxShadow: "0 24px 60px #0005",
          background: "#111b18",
        }}
      />
      <div
        style={{
          position: "absolute",
          left: 80,
          top: 1085,
          height: 5,
          width: 70,
          background: color,
          borderRadius: 4,
        }}
      />
      <div
        style={{
          position: "absolute",
          left: 80,
          top: 1475,
          right: 80,
          borderTop: "1px solid #66807144",
          paddingTop: 30,
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
      >
        {[
          ["instagram", "@bugra.code"],
          ["youtube", "@BugraAkdemir"],
          ["github", "BugraAkdemir"],
        ].map(([platform, name]) => (
          <div
            key={platform}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 17,
              fontSize: 30,
              color: "#c8d5cb",
            }}
          >
            <SocialLogo
              platform={platform as "instagram" | "youtube" | "github"}
              size={32}
            />
            <span>{name}</span>
          </div>
        ))}
      </div>
      <div
        style={{
          position: "absolute",
          left: 80,
          top: 1692,
          color: "#7c9586",
          fontSize: 24,
        }}
      >
        Tam anlatım YouTube’da · @BugraAkdemir
      </div>
    </AbsoluteFill>
  );
}
