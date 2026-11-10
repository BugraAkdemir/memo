import React from "react";
export const C = {
  bg: "#101719",
  paper: "#f4f0e6",
  muted: "#9daaa6",
  blue: "#68b8f7",
  yellow: "#f7d36d",
  orange: "#f39768",
  green: "#9fd9b7",
  line: "#35413f",
};
export function Icon({
  name,
  size = 120,
  color = C.blue,
}: {
  name: string;
  size?: number;
  color?: string;
}) {
  const paths: Record<string, React.ReactNode> = {
    file: (
      <>
        <path d="M26 10h42l25 25v72H26z" />
        <path d="M67 10v27h26M40 56h38M40 71h38M40 86h24" />
      </>
    ),
    chip: (
      <>
        <rect x="28" y="28" width="64" height="64" rx="12" />
        <rect x="43" y="43" width="34" height="34" rx="4" />
        {[36, 60, 84].map((x) => (
          <path key={x} d={`M${x} 12v16M${x} 92v16M12 ${x}h16M92 ${x}h16`} />
        ))}
      </>
    ),
    flask: (
      <>
        <path d="M43 12h34M49 12v35l-27 48q-6 13 10 13h56q16 0 10-13L71 47V12M35 80h51" />
        <circle cx="55" cy="66" r="3" />
        <circle cx="68" cy="92" r="4" />
      </>
    ),
    server: (
      <>
        <rect x="19" y="14" width="82" height="92" rx="12" />
        {[30, 60, 90].map((y) => (
          <g key={y}>
            <path d={`M32 ${y}h44`} />
            <circle cx="87" cy={y} r="2" />
          </g>
        ))}
      </>
    ),
    app: (
      <>
        <rect x="14" y="15" width="92" height="90" rx="12" />
        <path d="M14 37h92M31 26h1M43 26h1M55 26h1" />
        <rect x="29" y="53" width="24" height="33" rx="4" />
        <path d="M67 56h23M67 71h23M67 86h15" />
      </>
    ),
    brain: (
      <>
        <path d="M59 20c-16-15-37-2-34 16-18 4-19 29-6 38-8 17 9 31 22 24 6 17 19 10 19 2V26M61 20c16-15 37-2 34 16 18 4 19 29 6 38 8 17-9 31-22 24-6 17-19 10-19 2V26M35 37q19 0 16 17M26 74q14-13 24 0M84 37q-19 0-16 17M94 74q-14-13-24 0" />
      </>
    ),
    cloud: (
      <>
        <path d="M31 91h60c26 0 29-39 4-43C91 10 37 9 30 44 1 43 1 91 31 91z" />
        <path d="M60 49v30M47 66l13 13 13-13" />
      </>
    ),
    lock: (
      <>
        <rect x="23" y="49" width="74" height="58" rx="12" />
        <path d="M38 49V32a22 22 0 0 1 44 0v17M60 71v16" />
      </>
    ),
    shield: (
      <>
        <path d="M60 11l40 17v30q0 29-40 52Q20 87 20 58V28z" />
        <path d="M40 59l14 15 28-32" />
      </>
    ),
    clock: (
      <>
        <circle cx="60" cy="64" r="42" />
        <path d="M60 39v27l19 11M46 9h28M60 9v12" />
      </>
    ),
    tool: (
      <>
        <path d="M77 16a26 26 0 0 0-31 35L17 81a14 14 0 0 0 20 20l30-30a26 26 0 0 0 35-31L86 55 66 35z" />
      </>
    ),
    book: (
      <>
        <path d="M60 105V24Q34 8 13 20v76q23-11 47 9 24-20 47-9V20Q86 8 60 24M27 39l20 3M27 55l20 3M74 42l20-3M74 58l20-3" />
      </>
    ),
    money: (
      <>
        <rect x="10" y="29" width="100" height="64" rx="9" />
        <circle cx="60" cy="61" r="19" />
        <path d="M23 44h5M23 78h5M92 44h5M92 78h5M60 50v22" />
      </>
    ),
    box: (
      <>
        <path d="M60 12l45 25v51l-45 25-45-25V37zM15 37l45 25 45-25M60 62v51M38 24l45 25" />
      </>
    ),
    chart: (
      <>
        <path d="M16 12v93h94M36 87V61M59 87V43M82 87V25M31 43l25-20 22 9 24-21" />
      </>
    ),
    plug: (
      <>
        <path d="M42 10v26M78 10v26M29 37h62v17q0 29-31 29v25M29 51h62" />
      </>
    ),
    search: (
      <>
        <circle cx="50" cy="49" r="32" />
        <path d="M74 74l32 32" />
      </>
    ),
    check: <path d="M24 61l23 24 50-52" />,
    route: (
      <>
        <circle cx="25" cy="24" r="10" />
        <circle cx="96" cy="96" r="10" />
        <path d="M35 24h46q26 0 26 23t-30 23H45q-28 0-28 26h67" />
      </>
    ),
    screen: (
      <>
        <rect x="12" y="14" width="96" height="70" rx="8" />
        <path d="M44 84v19M76 84v19M30 104h60M26 65h68" />
      </>
    ),
    person: (
      <>
        <circle cx="60" cy="29" r="19" />
        <path d="M23 106V86q0-32 37-32t37 32v20" />
      </>
    ),
    puzzle: (
      <>
        <path d="M15 18h31q-7 25 14 25t14-25h31v31q-25-7-25 14t25 14v28H74q7-25-14-25t-14 25H15V77q25 7 25-14T15 49z" />
      </>
    ),
  };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      fill="none"
      stroke={color}
      strokeWidth="5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-label={name}
    >
      {paths[name] || paths.box}
    </svg>
  );
}

// Geometry and palettes adapted from frontend/lib/widgets/memo_mascot.dart.
// Original characters, rather than substitute robot illustrations.
export function Mascot({
  skin = "classic",
  pose = "point",
  frame = 0,
  size = 180,
}: {
  skin?: "classic" | "pixel";
  pose?: "point" | "think" | "wave" | "happy" | "talk";
  frame?: number;
  size?: number;
}) {
  const pixel = skin === "pixel",
    t = frame / 30,
    breathe = 1 - (0.018 * (1 - Math.cos(t * 2.4))) / 2;
  const blink = Math.sin(t * 1.4) > 0.995 ? 0.12 : 1;
  const eye = pixel ? "#7FE0EE" : "#E3A94B";
  const body = pixel ? "#3D6FE0" : "#4A3F35";
  const armAngle =
    pose === "wave"
      ? -125 + Math.sin(t * 4) * 12
      : pose === "talk"
        ? -70 + Math.sin(t * 5) * 12
        : pose === "point"
          ? -92
          : pose === "happy"
            ? -145
            : 15;
  const band = (
    cx: number,
    top: number,
    w: number,
    h: number,
    r: number,
    shade: number,
  ) =>
    Array.from({ length: Math.round(h / 6) }, (_, i) => {
      const bh = h / Math.round(h / 6),
        y = top + (i + 0.5) * bh,
        d = Math.min(y - top, top + h - y);
      const inset =
        d < r
          ? Math.min(
              w / 2 - 1,
              Math.ceil(
                (r * (1 - Math.sqrt(Math.max(0, 1 - (1 - d / r) ** 2)))) / 6,
              ) * 6,
            )
          : 0;
      return (
        <rect
          key={i}
          x={cx - w / 2 + inset}
          y={top + i * bh}
          width={w - 2 * inset}
          height={bh + 0.4}
          fill={y >= shade ? "#2A4FB8" : body}
        />
      );
    });
  return (
    <svg
      viewBox="0 0 200 210"
      width={size}
      height={size * 1.05}
      aria-label={pixel ? "Memo mavi robot maskotu" : "Memo kahverengi maskotu"}
      style={{ overflow: "visible" }}
    >
      <g transform={`translate(100 ${pixel ? 130 : 112})`}>
        <ellipse
          cy={pixel ? 66 : 78}
          rx="44"
          ry="9"
          fill="#000"
          opacity=".25"
        />
        <g transform={`scale(${2 - breathe} ${breathe})`}>
          {pixel ? (
            <>
              <rect x="-31" y="48" width="18" height="11" fill="#2A4FB8" />
              <rect x="13" y="48" width="18" height="11" fill="#2A4FB8" />
              {band(0, -97, 96, 82, 24, -30)}
              {band(0, -6, 78, 56, 16, 28)}
              <rect
                x="-38"
                y="-89"
                width="16"
                height="10"
                fill="#8FB4F5"
                opacity=".55"
              />
              <rect x="-5" y="13" width="10" height="10" fill={eye} />
              <rect x="-3" y="-121" width="6" height="24" fill="#2A4FB8" />
              <rect x="-5.5" y="-128" width="11" height="11" fill={eye} />
            </>
          ) : (
            <>
              <ellipse cx="-30" cy="57" rx="20" ry="11" fill="#362D25" />
              <ellipse cx="30" cy="57" rx="20" ry="11" fill="#362D25" />
              <path
                d="M0-60C-26-60-41-43-43-18C-48-9-48 21-39 34C-29 51-12 60 0 60C12 60 29 51 39 34C48 21 48-9 43-18C41-43 26-60 0-60"
                fill={body}
              />
              <ellipse
                cx="-15"
                cy="-38"
                rx="15"
                ry="11"
                fill="#5C4E40"
                opacity=".5"
              />
              <path
                d="M0-60V-83"
                stroke="#5C4E40"
                strokeWidth="6"
                strokeLinecap="round"
              />
              <circle cy="-87" r="7" fill={eye} />
              <ellipse cy="-8" rx="35" ry="30" fill="#362D25" opacity=".45" />
              <ellipse
                cx="-26"
                cy="6"
                rx="10"
                ry="5.5"
                fill="#D98B4F"
                opacity=".3"
              />
              <ellipse
                cx="26"
                cy="6"
                rx="10"
                ry="5.5"
                fill="#D98B4F"
                opacity=".3"
              />
            </>
          )}
          <g transform="translate(-44 0)">
            <rect
              x="-9"
              width="18"
              height="30"
              rx={pixel ? 0 : 9}
              fill={body}
            />
          </g>
          <g transform={`translate(43 0) rotate(${armAngle})`}>
            <rect
              x="-8.5"
              width="17"
              height="34"
              rx={pixel ? 0 : 8.5}
              fill={body}
            />
          </g>
          {pixel ? (
            <>
              <g>
                {band(0, -78, 62, 40, 9, 999).map((el, i) =>
                  React.cloneElement(el, { key: i, fill: "#0F1B33" }),
                )}
              </g>
              <g transform={`translate(0 -60) scale(1 ${blink})`}>
                <rect
                  x="-18"
                  y="-4.5"
                  width="9"
                  height={pose === "happy" ? 4 : 9}
                  fill={eye}
                />
                <rect
                  x="9"
                  y="-4.5"
                  width="9"
                  height={pose === "happy" ? 4 : 9}
                  fill={eye}
                />
              </g>
              <rect
                x="-7"
                y="-48.5"
                width="14"
                height={
                  pose === "talk" ? 3 + Math.abs(Math.sin(t * 17)) * 8 : 3
                }
                fill="#BFEFF5"
              />
            </>
          ) : (
            <>
              <g transform={`translate(0 -15) scale(1 ${blink})`}>
                <rect x="-21" y="-6" width="8" height="13" rx="4" fill={eye} />
                <rect x="13" y="-6" width="8" height="13" rx="4" fill={eye} />
              </g>
              {pose === "talk" ? (
                <ellipse
                  cy="19"
                  rx="7"
                  ry={2 + Math.abs(Math.sin(t * 17)) * 6}
                  fill="#2A211A"
                />
              ) : (
                <path
                  d="M-9 16Q0 24 9 16"
                  stroke="#2A211A"
                  strokeWidth="3"
                  strokeLinecap="round"
                  fill="none"
                />
              )}
            </>
          )}
        </g>
      </g>
    </svg>
  );
}

export function Python({ size = 150 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      aria-label="Python simgesi"
    >
      <path
        d="M59 8C29 8 28 16 28 29v13h32v5H15C-2 47-2 91 15 91h12V74c0-12 10-21 22-21h28c8 0 14-6 14-14V24C91 12 78 8 59 8Z"
        fill="#68b8f7"
      />
      <path
        d="M61 112c30 0 31-8 31-21V78H60v-5h45c17 0 17-44 0-44H93v17c0 12-10 21-22 21H43c-8 0-14 6-14 14v15c0 12 13 16 32 16Z"
        fill="#f7d36d"
      />
      <circle cx="42" cy="24" r="4" fill="#101719" />
      <circle cx="78" cy="96" r="4" fill="#101719" />
    </svg>
  );
}
export function Rust({ size = 150 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      aria-label="Rust simgesi"
    >
      <g fill={C.orange}>
        {Array.from({ length: 16 }, (_, i) => (
          <rect
            key={i}
            x="54"
            y="2"
            width="12"
            height="20"
            rx="1"
            transform={`rotate(${i * 22.5} 60 60)`}
          />
        ))}
        <circle cx="60" cy="60" r="45" />
      </g>
      <circle cx="60" cy="60" r="34" fill={C.bg} />
      <text
        x="59"
        y="82"
        textAnchor="middle"
        fill={C.orange}
        fontSize="65"
        fontFamily="Georgia"
        fontWeight="bold"
      >
        R
      </text>
    </svg>
  );
}
