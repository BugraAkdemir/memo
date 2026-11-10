import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { Input, ALL_FORMATS, FilePathSource } from "mediabunny";
const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const spec = JSON.parse(
  await fs.readFile(path.join(root, "script/narration.json"), "utf8"),
);
const timings = JSON.parse(
  await fs.readFile(path.join(root, "src/voice-film.json"), "utf8"),
);
const beats = spec.beats.filter((b) => !["s00-p0", "s00-p1"].includes(b.id));
const out = path.join(root, "out/audio-parts");
await fs.mkdir(out, { recursive: true });
let frame = 0;
const timeline = [];
let lastScene = -1;
function run(args) {
  const r = spawnSync(
    "ffmpeg",
    ["-v", "error", "-nostdin", "-threads", "1", ...args],
    { encoding: "utf8" },
  );
  if (r.status !== 0) throw new Error(r.stderr);
}
const list = [];
for (const b of beats) {
  const src = path.join(root, `public/voice/tr/${b.id}.mp3`);
  const input = new Input({
    formats: ALL_FORMATS,
    source: new FilePathSource(src),
  });
  const sec = await input.computeDuration();
  input.dispose();
  if (Math.abs(sec - timings[b.id].sec) > 0.12)
    throw new Error("Duration disagreement " + b.id);
  const lead = b.scene !== lastScene ? 10 : 5,
    tail = 6;
  const duration = Math.ceil(sec * 30) + lead + tail;
  const dest = path.join(out, b.id + ".wav");
  run([
    "-i",
    src,
    "-af",
    `adelay=${(lead * 1000) / 30}:all=1,apad,atrim=duration=${duration / 30}`,
    "-ar",
    "44100",
    "-ac",
    "1",
    "-c:a",
    "pcm_s16le",
    "-y",
    dest,
  ]);
  list.push(`file '${dest}'`);
  timeline.push({
    ...b,
    from: frame,
    duration,
    lead,
    voiceSec: sec,
    chapterStart: b.scene !== lastScene,
  });
  frame += duration;
  lastScene = b.scene;
}
await fs.writeFile(path.join(out, "concat.txt"), list.join("\n") + "\n");
run([
  "-f",
  "concat",
  "-safe",
  "0",
  "-i",
  path.join(out, "concat.txt"),
  "-c:a",
  "pcm_s16le",
  "-y",
  path.join(root, "public/voice/film-narration.wav"),
]);
const result = {
  fps: 30,
  width: 1920,
  height: 1080,
  totalFrames: frame,
  totalSeconds: frame / 30,
  beats: timeline,
};
if (frame < 360 * 30) throw new Error("Film is shorter than six minutes");
await fs.writeFile(
  path.join(root, "src/film-timeline.json"),
  JSON.stringify(result, null, 2) + "\n",
);
const nodes = [
  "// Generated from measured narration. Every beat has a separate source node.",
  "import {Sequence,useVideoConfig} from 'remotion';",
  "import timeline from '../film-timeline.json';",
  "import {Shot} from './Shot';",
  "export function AuthoredTimeline(){const {fps}=useVideoConfig();return <>",
];
for (let i = 0; i < timeline.length; i++) {
  const b = timeline[i];
  nodes.push(
    `<Sequence name=${JSON.stringify(b.id)} from={${b.from}} durationInFrames={${b.duration}} premountFor={fps}><Shot beat={timeline.beats[${i}]}/></Sequence>`,
  );
}
nodes.push("</>;}");
await fs.writeFile(
  path.join(root, "src/film/timeline-nodes.tsx"),
  nodes.join("\n") + "\n",
);
console.log(`MEDIABUNNY_FFPROBE_AGREE ${beats.length}/${beats.length}`);
console.log(`FILM_TIMELINE ${frame} frames / ${(frame / 30).toFixed(2)}s`);
