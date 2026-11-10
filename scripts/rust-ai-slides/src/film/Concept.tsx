import React from "react";
import { interpolate, useCurrentFrame, useVideoConfig } from "remotion";
import {
  Backdrop,
  C,
  Heading,
  Icon,
  MotionCard,
  Pill,
  Pop,
  clamp,
} from "./shared";
import { Python, Rust } from "../visuals";
type Props = { kind: string; part: number; mode: string };
const rows: Record<
  string,
  {
    title: string;
    items: { icon: string; title: string; label: string; color: string }[];
  }
> = {
  three: {
    title: "Üç ayrı iş. Üç ayrı ihtiyaç.",
    items: [
      {
        icon: "flask",
        title: "Modeli geliştir",
        label: "Veri + eğitim",
        color: C.blue,
      },
      {
        icon: "brain",
        title: "Modeli çalıştır",
        label: "Girdi → cevap",
        color: C.yellow,
      },
      {
        icon: "app",
        title: "Ürünü kur",
        label: "Kullanıcı + altyapı",
        color: C.orange,
      },
    ],
  },
  layers: {
    title: "Aynı uygulamanın farklı katmanları",
    items: [
      {
        icon: "flask",
        title: "Tarifi geliştir",
        label: "Model üzerinde çalış",
        color: C.blue,
      },
      {
        icon: "chip",
        title: "Yemeği pişir",
        label: "Modeli çalıştır",
        color: C.yellow,
      },
      {
        icon: "app",
        title: "Restoranı işlet",
        label: "Ürünün altyapısı",
        color: C.orange,
      },
    ],
  },
  rust: {
    title: "Rust’ın sunduğu kontrol",
    items: [
      {
        icon: "tool",
        title: "Kaynak kontrolü",
        label: "Belleği nasıl kullanıyorum?",
        color: C.orange,
      },
      {
        icon: "shield",
        title: "Bellek güvenliği",
        label: "Kuralları erken kontrol et",
        color: C.green,
      },
      {
        icon: "route",
        title: "Eşzamanlı işler",
        label: "Veriyi güvenle paylaş",
        color: C.blue,
      },
    ],
  },
  fit: {
    title: "Senin projen için uygun mu?",
    items: [
      {
        icon: "brain",
        title: "Model",
        label: "Destekleniyor mu?",
        color: C.blue,
      },
      { icon: "chip", title: "Donanım", label: "Uyumlu mu?", color: C.yellow },
      {
        icon: "puzzle",
        title: "Özellikler",
        label: "Gereken araçlar var mı?",
        color: C.orange,
      },
    ],
  },
  measure: {
    title: "Önce ölç. Sonra değiştir.",
    items: [
      {
        icon: "clock",
        title: "Başlangıcı ölç",
        label: "Aynı iş yükü",
        color: C.blue,
      },
      {
        icon: "tool",
        title: "Bir şeyi değiştir",
        label: "Kontrollü deney",
        color: C.orange,
      },
      {
        icon: "chart",
        title: "Yeniden ölç",
        label: "Aynı koşullar",
        color: C.green,
      },
    ],
  },
  decision: {
    title: "Kararın, yaptığın işe bağlı.",
    items: [
      {
        icon: "book",
        title: "Yeni başlıyorum",
        label: "Önerim: Python ile başla",
        color: C.blue,
      },
      {
        icon: "app",
        title: "Ürünüm çalışıyor",
        label: "Önce sorunu ölç",
        color: C.yellow,
      },
      {
        icon: "tool",
        title: "Sistem kuruyorum",
        label: "Rust’ı değerlendir",
        color: C.orange,
      },
    ],
  },
  candle: {
    title: "Rust ile gerçek model çalıştırma",
    items: [
      {
        icon: "file",
        title: "Metin",
        label: "Metin üretimi örnekleri",
        color: C.blue,
      },
      {
        icon: "plug",
        title: "Ses",
        label: "Ses tanıma örnekleri",
        color: C.yellow,
      },
      {
        icon: "screen",
        title: "Görüntü",
        label: "Görüntü işleme örnekleri",
        color: C.orange,
      },
    ],
  },
};
export function Concept({ kind, part, mode }: Props) {
  const f = useCurrentFrame(),
    { fps } = useVideoConfig();
  const phase = Math.floor(f / (4.2 * fps));
  if (kind === "opening")
    return (
      <Backdrop accent={C.yellow}>
        <Pop delay={0.1} style={{ position: "absolute", left: 105, top: 185 }}>
          <Heading>
            {phase === 0
              ? "PYTHON\nYETMİYOR MU?"
              : phase === 1
                ? "YANLIŞ DİLİ Mİ\nÖĞRENİYORUM?"
                : "ÖNCE YAPTIĞIN\nİŞE BAK."}
          </Heading>
        </Pop>
        <div style={{ position: "absolute", left: 1100, top: 250 }}>
          <Pop delay={0.3} x={100}>
            <div style={{ rotate: `${Math.sin(f / (fps * 3)) * 5}deg` }}>
              <Python size={300} />
            </div>
          </Pop>
        </div>
        <div style={{ position: "absolute", right: 180, top: 610 }}>
          <Pop delay={0.6} x={-100}>
            <div style={{ rotate: `${Math.sin(f / (fps * 4)) * -6}deg` }}>
              <Rust size={260} />
            </div>
          </Pop>
        </div>
        <Pop
          delay={0.9}
          style={{
            position: "absolute",
            left: 110,
            top: 740,
            display: "flex",
            gap: 25,
          }}
        >
          <Pill>Model</Pill>
          <Pill color={C.blue}>Uygulama</Pill>
          <Pill color={C.orange}>Altyapı</Pill>
        </Pop>
      </Backdrop>
    );
  if (kind === "keys")
    return (
      <Backdrop accent={C.yellow}>
        <Pop style={{ position: "absolute", top: 140, left: 100 }}>
          <Heading small>Bir anahtar. İki kişi.</Heading>
        </Pop>
        <div style={{ position: "absolute", left: 250, top: 420 }}>
          <Pop>
            <Icon name="person" size={200} color={C.blue} />
            <Pill color={C.blue}>Birinci iş</Pill>
          </Pop>
        </div>
        <div style={{ position: "absolute", right: 250, top: 420 }}>
          <Pop delay={0.5}>
            <Icon name="person" size={200} color={C.orange} />
            <Pill color={C.orange}>İkinci iş</Pill>
          </Pop>
        </div>
        <div
          style={{
            position: "absolute",
            left: interpolate(
              f % (7 * fps),
              [0, 2.5 * fps, 4 * fps, 7 * fps],
              [650, 1050, 1050, 650],
              clamp,
            ),
            top: 490,
          }}
        >
          <svg width="250" height="150" viewBox="0 0 250 150">
            <circle
              cx="60"
              cy="65"
              r="38"
              fill="none"
              stroke={C.yellow}
              strokeWidth="16"
            />
            <path
              d="M98 65h105v35m-37-35v27"
              fill="none"
              stroke={C.yellow}
              strokeWidth="16"
            />
          </svg>
        </div>
        <Pop delay={1} style={{ position: "absolute", left: 620, top: 300 }}>
          <Pill>Değiştirme izni kimde?</Pill>
        </Pop>
        <Pop
          delay={2}
          style={{
            position: "absolute",
            left: 480,
            top: 780,
            display: "flex",
            alignItems: "center",
            gap: 22,
          }}
        >
          <Icon name="shield" color={C.green} size={80} />
          <span style={{ fontSize: 35, fontWeight: 800 }}>
            Bazı hataları derlerken yakala.
          </span>
        </Pop>
        <span
          style={{
            position: "absolute",
            left: 105,
            bottom: 45,
            fontSize: 22,
            color: C.muted,
          }}
        >
          Sahiplik ve ödünç alma için basitleştirilmiş benzetme
        </span>
      </Backdrop>
    );
  if (kind === "safety")
    return (
      <Backdrop accent={C.green}>
        <Pop style={{ position: "absolute", top: 150, left: 105 }}>
          <Heading small>
            Güvenlik kuralları, doğru mantığın yerini tutmaz.
          </Heading>
        </Pop>
        <div
          style={{
            position: "absolute",
            left: 170,
            top: 420,
            display: "flex",
            alignItems: "center",
            gap: 100,
          }}
        >
          <MotionCard
            icon="shield"
            title="Erken uyarı"
            label="Bazı hata türlerini engelle"
            color={C.green}
          />
          <span style={{ fontSize: 150, color: C.yellow }}>≠</span>
          <MotionCard
            icon="brain"
            title="Hatasız uygulama"
            label="Mantık hataları hâlâ mümkün"
            color={C.orange}
            index={1}
          />
        </div>
      </Backdrop>
    );
  if (kind === "cost")
    return (
      <Backdrop accent={C.orange}>
        <Pop style={{ position: "absolute", top: 145, left: 105 }}>
          <Heading small>Senin zamanın da bir kaynak.</Heading>
        </Pop>
        <div
          style={{
            position: "absolute",
            left: 190,
            top: 365,
            display: "flex",
            alignItems: "center",
            gap: 100,
          }}
        >
          <MotionCard
            icon="chip"
            title="Çalıştırma"
            label="Bellek + işlemci + donanım"
            color={C.blue}
          />
          <span style={{ fontSize: 110, color: C.yellow }}>+</span>
          <MotionCard
            icon="clock"
            title="Geliştirme"
            label="Öğrenme + entegrasyon + bakım"
            color={C.orange}
            index={1}
          />
        </div>
        <Pop delay={1.7} style={{ position: "absolute", left: 195, top: 850 }}>
          <Pill color={C.orange}>Fayda, geçiş maliyetine değiyor mu?</Pill>
        </Pop>
      </Backdrop>
    );
  if (kind === "ending")
    return (
      <Backdrop accent={C.green}>
        <Pop style={{ position: "absolute", left: 105, top: 190 }}>
          <Heading>{"HANGİ PROBLEMİ\nÇÖZÜYORUM?"}</Heading>
        </Pop>
        <Pop
          delay={0.5}
          style={{
            position: "absolute",
            left: 125,
            top: 560,
            display: "flex",
            alignItems: "center",
            gap: 55,
          }}
        >
          <Python size={155} />
          <span style={{ fontSize: 95, color: C.yellow }}>+</span>
          <Rust size={155} />
        </Pop>
        <Pop delay={1.2} style={{ position: "absolute", left: 125, top: 815 }}>
          <Pill>Sen ne geliştiriyorsun? Yorumlara yaz.</Pill>
        </Pop>
      </Backdrop>
    );
  const row = rows[kind] || rows.three;
  const active = (phase + part) % 3;
  return (
    <Backdrop accent={row.items[active].color}>
      <div style={{ position: "absolute", left: 105, top: 145, right: 105 }}>
        <Pop>
          <Heading small>{row.title}</Heading>
        </Pop>
      </div>
      {kind === "candle" && (
        <Pop
          delay={0.2}
          style={{
            position: "absolute",
            top: 280,
            left: 110,
            display: "flex",
            gap: 22,
            alignItems: "center",
          }}
        >
          <Rust size={75} />
          <span style={{ fontSize: 48, fontWeight: 800 }}>Candle</span>
          <Pill color={C.orange}>Hugging Face</Pill>
        </Pop>
      )}
      <div
        style={{
          position: "absolute",
          left: 150,
          right: 150,
          top: kind === "candle" ? 405 : 370,
          display: "flex",
          gap: 65,
          justifyContent: "center",
        }}
      >
        {row.items.map((x, i) => (
          <MotionCard
            {...x}
            key={x.title}
            active={phase < 1 || i === active}
            index={i}
          />
        ))}
      </div>
      <Pop
        delay={1.7}
        style={{
          position: "absolute",
          left: 155,
          top: kind === "candle" ? 860 : 835,
        }}
      >
        <Pill color={row.items[active].color}>
          {kind === "decision"
            ? "Tek reçete yok. İhtiyacına göre seç."
            : kind === "measure"
              ? "Ölçülmüş sonuç olmadan hız çarpanı yok."
              : kind === "candle"
                ? "Desteklenen model ve donanımı ayrıca kontrol et."
                : kind === "rust"
                  ? "Daha fazla kontrol, daha fazla sorumluluk."
                  : "Aynı üründe farklı diller birlikte çalışabilir."}
        </Pill>
      </Pop>
    </Backdrop>
  );
}
