import http from "node:http";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { context } from "esbuild";
const root = path.dirname(fileURLToPath(import.meta.url));
const dist = path.join(root, "dist");
await fs.mkdir(dist, { recursive: true });
const ctx = await context({
  entryPoints: [path.join(root, "src/app.tsx")],
  bundle: true,
  outdir: dist,
  platform: "browser",
  format: "esm",
  jsx: "automatic",
  sourcemap: true,
  loader: { ".woff2": "file" },
});
await ctx.rebuild();
await ctx.watch();
const types = {
  ".js": "text/javascript",
  ".css": "text/css",
  ".html": "text/html",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".woff2": "font/woff2",
};
const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://localhost");
    let base = dist,
      rel = url.pathname.slice(1);
    if (["art/", "voice/", "sfx/"].some((prefix) => rel.startsWith(prefix)))
      base = path.join(root, "public");
    if (!rel) rel = "index.html";
    const file = path.resolve(base, rel);
    if (!file.startsWith(base + path.sep)) {
      res.writeHead(403);
      res.end();
      return;
    }
    const content =
      rel === "index.html"
        ? `<!doctype html><html lang="tr"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Python × Rust — Seslendirmeli Film</title><link rel="stylesheet" href="/app.css"><div id="root"></div><script type="module" src="/app.js"></script></html>`
        : await fs.readFile(file);
    res.writeHead(200, {
      "Content-Type": types[path.extname(file)] || "application/octet-stream",
      "Cache-Control": "no-cache",
    });
    res.end(content);
  } catch {
    res.writeHead(404);
    res.end("Dosya bulunamadı");
  }
});
server.listen(4317, "127.0.0.1", () =>
  console.log("Video: http://127.0.0.1:4317"),
);
for (const sig of ["SIGTERM", "SIGINT"])
  process.on(sig, async () => {
    server.close();
    await ctx.dispose();
    process.exit();
  });
