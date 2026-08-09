// Memo — minimal web client. No build step, no framework: talks to the same
// REST/SSE API the desktop app and CLI use. Scope is deliberately narrow —
// chat plus the handful of settings you need before the full desktop app
// (which can already manage everything else remotely, see AGENTS.md) is
// reachable: starting/stopping/importing/downloading the local model,
// connecting a provider, managing accounts.

const TOKEN_KEY = "memo_web_token";

function getToken() { return localStorage.getItem(TOKEN_KEY) || ""; }
function setToken(t) { localStorage.setItem(TOKEN_KEY, t); }
function clearToken() { localStorage.removeItem(TOKEN_KEY); }

// api(): fetch wrapper. Adds X-Memo-Token when we have one; on 401 clears
// the stored token and throws a typed error so callers can show the login
// screen instead of a generic failure message.
async function api(path, opts = {}) {
  const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
  const token = getToken();
  if (token) headers["X-Memo-Token"] = token;
  const res = await fetch(path, Object.assign({}, opts, { headers }));
  if (res.status === 401) {
    clearToken();
    const err = new Error("unauthorized");
    err.unauthorized = true;
    throw err;
  }
  return res;
}

async function apiJSON(path, opts = {}) {
  const res = await api(path, opts);
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(text || `${res.status} ${res.statusText}`);
  }
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json")) return res.json();
  return null;
}

// ── generic UI helpers: reveal-toggle fields, modals ────────────────────

// Any input wrapped in .reveal-field with a sibling [data-target] button
// gets a show/hide eye toggle — used on every password/token/API-key field
// on this page so what you typed is always verifiable before you submit
// it, not just trusted blind.
function wireRevealToggles() {
  document.querySelectorAll(".reveal-toggle").forEach((btn) => {
    btn.addEventListener("click", () => {
      const input = document.getElementById(btn.dataset.target);
      if (!input) return;
      const showing = input.type === "text";
      input.type = showing ? "password" : "text";
      btn.textContent = showing ? "👁" : "🙈";
      btn.setAttribute("aria-label", showing ? "Show" : "Hide");
    });
  });
}

function openModal(id) { document.getElementById(id).classList.remove("hidden"); }
function closeModal(id) { document.getElementById(id).classList.add("hidden"); }

function wireModalCloseButtons() {
  document.querySelectorAll("[data-close-modal]").forEach((btn) => {
    btn.addEventListener("click", () => closeModal(btn.dataset.closeModal));
  });
  document.querySelectorAll(".modal-backdrop").forEach((backdrop) => {
    backdrop.addEventListener("click", (e) => {
      if (e.target === backdrop) backdrop.classList.add("hidden");
    });
  });
}

// ── boot ─────────────────────────────────────────────────────────────────
//
// Faz 5.1 (yapacam.md): before probing /api/status the usual way, check
// whether this server has ever had an account/password configured at all.
// A brand new install (curl installer just finished, nobody has touched
// Settings yet) shows the first-run "create your admin account" screen
// instead of a token-entry prompt no one has a token for yet — the whole
// point of this feature. /api/setup/status is deliberately unauthenticated
// (see isSetupBootstrapPath, server-side), so this call always succeeds
// regardless of auth mode.

let authMode = "";

async function boot() {
  wireRevealToggles();
  wireModalCloseButtons();

  try {
    const setup = await apiJSON("/api/setup/status");
    if (setup && setup.needs_setup) {
      showSetup();
      return;
    }
    authMode = (setup && setup.auth_mode) || "";
  } catch (e) {
    // Setup-status itself unreachable (backend not up yet) — fall through
    // to the normal probe below exactly as before this feature existed.
  }

  try {
    await apiJSON("/api/status");
    showApp();
  } catch (e) {
    if (e.unauthorized) {
      showLogin();
    } else {
      // Local-only mode (no token required) but the backend itself isn't
      // answering yet, or some other transient error — still try the app
      // shell; individual panels show their own errors.
      showApp();
    }
  }
}

// Both login methods are always offered, regardless of the detected
// auth_mode — only the *default* tab follows it. A wrong/stale auth_mode
// detection (a transient network hiccup on the /api/setup/status probe
// above, for instance) used to hide the one method that would have
// actually worked; now the user can just click the other tab instead of
// being stuck looking at a form for a credential they don't have.
function showLogin() {
  document.getElementById("setup-screen").classList.add("hidden");
  document.getElementById("login-screen").classList.remove("hidden");
  document.getElementById("app").classList.add("hidden");
  document.getElementById("login-error").classList.add("hidden");

  setLoginTab(authMode === "token" ? "token" : "password");
}

function setLoginTab(which) {
  const isPassword = which === "password";
  document.getElementById("login-tab-password").classList.toggle("active", isPassword);
  document.getElementById("login-tab-token").classList.toggle("active", !isPassword);
  document.getElementById("password-login-block").classList.toggle("hidden", !isPassword);
  document.getElementById("token-login-block").classList.toggle("hidden", isPassword);
  const focusTarget = document.getElementById(isPassword ? "login-username" : "token-input");
  if (focusTarget) focusTarget.focus();
}
document.getElementById("login-tab-password").addEventListener("click", () => setLoginTab("password"));
document.getElementById("login-tab-token").addEventListener("click", () => setLoginTab("token"));

function showSetup() {
  document.getElementById("login-screen").classList.add("hidden");
  document.getElementById("app").classList.add("hidden");
  document.getElementById("setup-screen").classList.remove("hidden");
  document.getElementById("setup-username").focus();
}

function showApp() {
  document.getElementById("setup-screen").classList.add("hidden");
  document.getElementById("login-screen").classList.add("hidden");
  document.getElementById("app").classList.remove("hidden");
  initApp();
}

document.getElementById("token-submit").addEventListener("click", async () => {
  const val = document.getElementById("token-input").value.trim();
  const errEl = document.getElementById("login-error");
  errEl.classList.add("hidden");
  if (!val) return;
  setToken(val);
  try {
    await apiJSON("/api/status");
    showApp();
  } catch (e) {
    clearToken();
    errEl.textContent = "That token was rejected — check it and try again.";
    errEl.classList.remove("hidden");
  }
});
document.getElementById("token-input").addEventListener("keydown", (e) => {
  if (e.key === "Enter") document.getElementById("token-submit").click();
});

document.getElementById("login-submit").addEventListener("click", async () => {
  const username = document.getElementById("login-username").value.trim();
  const password = document.getElementById("login-password").value;
  const errEl = document.getElementById("login-error");
  errEl.classList.add("hidden");
  if (!username || !password) {
    errEl.textContent = "Enter both a username and a password.";
    errEl.classList.remove("hidden");
    return;
  }
  try {
    const res = await fetch("/api/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(text || "invalid credentials");
    }
    const data = await res.json();
    setToken(data.session_token);
    showApp();
  } catch (e) {
    errEl.textContent = "Login failed — check your username and password.";
    errEl.classList.remove("hidden");
  }
});
["login-username", "login-password"].forEach((id) => {
  document.getElementById(id).addEventListener("keydown", (e) => {
    if (e.key === "Enter") document.getElementById("login-submit").click();
  });
});

document.getElementById("setup-submit").addEventListener("click", async () => {
  const username = document.getElementById("setup-username").value.trim();
  const password = document.getElementById("setup-password").value;
  const confirmPw = document.getElementById("setup-password-confirm").value;
  const errEl = document.getElementById("setup-error");
  errEl.classList.add("hidden");
  if (!username || !password) {
    errEl.textContent = "Username and password are required.";
    errEl.classList.remove("hidden");
    return;
  }
  if (password !== confirmPw) {
    errEl.textContent = "Passwords don't match.";
    errEl.classList.remove("hidden");
    return;
  }
  try {
    const res = await fetch("/api/setup/create-admin", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(text || `${res.status} ${res.statusText}`);
    }
    const data = await res.json();
    setToken(data.session_token);
    showApp();
  } catch (e) {
    errEl.textContent = "Could not create account: " + e.message;
    errEl.classList.remove("hidden");
  }
});
["setup-username", "setup-password", "setup-password-confirm"].forEach((id) => {
  document.getElementById(id).addEventListener("keydown", (e) => {
    if (e.key === "Enter") document.getElementById("setup-submit").click();
  });
});

// ── app shell (tabs, status dot) ────────────────────────────────────────

let appInited = false;

function initApp() {
  if (appInited) return;
  appInited = true;

  document.querySelectorAll(".tab").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".tab").forEach((b) => b.classList.remove("active"));
      document.querySelectorAll(".view").forEach((v) => v.classList.remove("active"));
      btn.classList.add("active");
      document.getElementById(btn.dataset.tab + "-view").classList.add("active");
      if (btn.dataset.tab === "settings") loadSettings();
    });
  });

  pollStatus();
  setInterval(pollStatus, 15000);

  loadMessages();
  document.getElementById("chat-form").addEventListener("submit", onSend);
  const ta = document.getElementById("chat-text");
  ta.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      document.getElementById("chat-form").requestSubmit();
    }
  });

  document.getElementById("model-start").addEventListener("click", startModel);
  document.getElementById("model-stop").addEventListener("click", stopModel);
  document.getElementById("model-browse-import").addEventListener("click", openImportModelBrowser);
  document.getElementById("model-search-open").addEventListener("click", openHFSearch);
  document.getElementById("provider-form").addEventListener("submit", saveProvider);
  document.getElementById("provider-cancel-edit").addEventListener("click", cancelProviderEdit);
  document.getElementById("p-test").addEventListener("click", testProvider);
  document.getElementById("p-fetch-models").addEventListener("click", fetchModels);
  document.getElementById("restart-backend").addEventListener("click", restartBackend);
  document.getElementById("account-form").addEventListener("submit", saveAccount);
  document.getElementById("hf-search-btn").addEventListener("click", runHFSearch);
  document.getElementById("hf-query").addEventListener("keydown", (e) => {
    if (e.key === "Enter") runHFSearch();
  });
}

async function pollStatus() {
  const dot = document.getElementById("status-dot");
  try {
    await apiJSON("/api/status");
    dot.className = "status-dot ok";
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    dot.className = "status-dot error";
  }
}

// ── chat ─────────────────────────────────────────────────────────────────

const messagesEl = document.getElementById("messages");

function addBubble(role, text) {
  const div = document.createElement("div");
  div.className = "bubble " + role;
  div.textContent = text;
  messagesEl.appendChild(div);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return div;
}

async function loadMessages() {
  try {
    const msgs = await apiJSON("/api/messages");
    messagesEl.innerHTML = "";
    (msgs || []).forEach((m) => {
      if (m.role === "user" || m.role === "assistant") {
        addBubble(m.role, typeof m.content === "string" ? m.content : JSON.stringify(m.content));
      }
    });
  } catch (e) {
    if (e.unauthorized) showLogin();
    // Empty history on a fresh chat isn't an error — leave the pane blank.
  }
}

let sending = false;

async function onSend(ev) {
  ev.preventDefault();
  if (sending) return;
  const ta = document.getElementById("chat-text");
  const text = ta.value.trim();
  if (!text) return;
  ta.value = "";
  addBubble("user", text);

  const sendBtn = document.getElementById("chat-send");
  sending = true;
  sendBtn.disabled = true;

  const replyBubble = addBubble("assistant", "");
  let replyText = "";

  try {
    const res = await api("/api/send/stream", {
      method: "POST",
      body: JSON.stringify({ message: text }),
    });
    if (!res.ok || !res.body) {
      throw new Error(`${res.status} ${res.statusText}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop(); // last (possibly partial) line stays in buf
      for (const line of lines) {
        if (!line.startsWith("data:")) continue;
        const payload = line.slice(5).trim();
        if (!payload) continue;
        let chunk;
        try { chunk = JSON.parse(payload); } catch { continue; }
        if (chunk.error) {
          replyBubble.classList.add("error");
          replyText += (replyText ? "\n" : "") + chunk.error;
          replyBubble.textContent = replyText;
        } else if (chunk.content) {
          replyText += chunk.content;
          replyBubble.textContent = replyText;
          messagesEl.scrollTop = messagesEl.scrollHeight;
        }
        if (chunk.done) break;
      }
    }
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    replyBubble.classList.add("error");
    replyBubble.textContent = "Connection error: " + e.message;
  } finally {
    sending = false;
    sendBtn.disabled = false;
  }
}

// ── settings: local model ───────────────────────────────────────────────

let settingsLoaded = false;

async function loadSettings() {
  if (settingsLoaded) return;
  settingsLoaded = true;
  await Promise.all([loadModels(), loadProviders(), loadRemoteAccess(), loadAccounts()]);
}

async function loadModels() {
  const statusEl = document.getElementById("model-status");
  const select = document.getElementById("model-select");
  try {
    const [models, status] = await Promise.all([
      apiJSON("/api/models/local"),
      apiJSON("/api/models/status"),
    ]);
    select.innerHTML = "";
    (models || []).forEach((m) => {
      const opt = document.createElement("option");
      opt.value = m.path;
      opt.textContent = m.filename || m.repo_id || m.path;
      select.appendChild(opt);
    });
    if (status && status.running) {
      statusEl.textContent = `Running — ${status.model_name || status.model_path} (port ${status.port})`;
      if (status.model_path) select.value = status.model_path;
    } else {
      statusEl.textContent = (models && models.length) ? "Not running." : "No local models yet — import one from the server's disk, or download one from Hugging Face below.";
    }
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    statusEl.textContent = "Could not load model status: " + e.message;
  }
}

async function startModel() {
  const select = document.getElementById("model-select");
  const path = select.value;
  if (!path) return;
  const statusEl = document.getElementById("model-status");
  statusEl.textContent = "Starting…";
  try {
    await apiJSON("/api/models/start", {
      method: "POST",
      body: JSON.stringify({ path, ctx_size: 4096, port: 8081, gpu_layers: -1 }),
    });
    await loadModels();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    statusEl.textContent = "Failed to start: " + e.message;
  }
}

async function stopModel() {
  const statusEl = document.getElementById("model-status");
  statusEl.textContent = "Stopping…";
  try {
    await apiJSON("/api/models/stop", { method: "POST" });
    await loadModels();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    statusEl.textContent = "Failed to stop: " + e.message;
  }
}

// ── settings: server-side file browser (model import) ──────────────────
//
// Browses the *backend's own* filesystem — not whatever device this page
// happens to be open on. A model file has to already exist on the server
// (llama.cpp runs there, not in this browser tab), so a plain HTML
// <input type="file"> — which only ever sees the local device's disk —
// would silently pick the wrong thing on any non-local setup. This talks
// to GET /api/files/browse (same endpoint the desktop app's agent/CLI
// folder pickers now use, for the identical reason) instead.

let browseState = { path: "", parent: "", onPick: null };

async function openImportModelBrowser() {
  document.getElementById("browse-modal-title").textContent = "Import a model file";
  browseState.onPick = async (path) => {
    closeModal("browse-modal");
    const statusEl = document.getElementById("model-status");
    statusEl.textContent = "Importing…";
    try {
      await apiJSON("/api/models/import", { method: "POST", body: JSON.stringify({ path }) });
      await loadModels();
    } catch (e) {
      if (e.unauthorized) { showLogin(); return; }
      statusEl.textContent = "Import failed: " + e.message;
    }
  };
  openModal("browse-modal");
  await browseTo("");
}

async function browseTo(path) {
  const body = document.getElementById("browse-body");
  const pathEl = document.getElementById("browse-path");
  const upBtn = document.getElementById("browse-up");
  body.innerHTML = '<p class="modal-loading">Loading…</p>';
  try {
    const result = await apiJSON("/api/files/browse?path=" + encodeURIComponent(path));
    browseState.path = result.path;
    browseState.parent = result.parent || "";
    pathEl.textContent = result.path;
    pathEl.title = result.path;
    upBtn.disabled = !browseState.parent;
    body.innerHTML = "";
    if (!result.entries || !result.entries.length) {
      body.innerHTML = '<p class="modal-empty">This folder is empty.</p>';
      return;
    }
    result.entries.forEach((entry) => {
      const row = document.createElement("div");
      row.className = "browse-entry" + (entry.is_dir ? " is-dir" : "");
      const childPath = browseState.path.endsWith("/") ? browseState.path + entry.name : browseState.path + "/" + entry.name;
      row.innerHTML = `
        <span class="icon">${entry.is_dir ? "📁" : "📄"}</span>
        <span class="entry-name">${escapeHTML(entry.name)}</span>
        <span class="entry-size">${entry.is_dir ? "" : formatBytes(entry.size)}</span>
      `;
      row.addEventListener("click", () => {
        if (entry.is_dir) {
          browseTo(childPath);
        } else if (browseState.onPick) {
          browseState.onPick(childPath);
        }
      });
      body.appendChild(row);
    });
  } catch (e) {
    if (e.unauthorized) { closeModal("browse-modal"); showLogin(); return; }
    body.innerHTML = '<p class="modal-empty">Could not read this folder: ' + escapeHTML(e.message) + "</p>";
  }
}
document.getElementById("browse-up").addEventListener("click", () => {
  if (browseState.parent) browseTo(browseState.parent);
});

function formatBytes(n) {
  if (!n) return "";
  if (n >= 1024 * 1024 * 1024) return (n / (1024 * 1024 * 1024)).toFixed(1) + " GB";
  if (n >= 1024 * 1024) return (n / (1024 * 1024)).toFixed(1) + " MB";
  if (n >= 1024) return (n / 1024).toFixed(0) + " KB";
  return n + " B";
}

// ── settings: Hugging Face search + download ────────────────────────────

let downloadPollTimer = null;

function openHFSearch() {
  document.getElementById("hf-body").innerHTML = '<p class="modal-empty">Search for a model above (e.g. a family name + "gguf").</p>';
  document.getElementById("hf-query").value = "";
  openModal("hf-modal");
  document.getElementById("hf-query").focus();
  startDownloadPolling();
}

async function runHFSearch() {
  const query = document.getElementById("hf-query").value.trim();
  const body = document.getElementById("hf-body");
  if (!query) return;
  body.innerHTML = '<p class="modal-loading">Searching…</p>';
  try {
    const results = await apiJSON("/api/models/search", { method: "POST", body: JSON.stringify({ query }) });
    if (!results || !results.length) {
      body.innerHTML = '<p class="modal-empty">No results.</p>';
      return;
    }
    body.innerHTML = "";
    results.forEach((r) => {
      const row = document.createElement("div");
      row.className = "hf-result";
      row.innerHTML = `
        <span class="hf-name">${escapeHTML(r.id)}</span>
        <span class="hf-meta">⬇ ${r.downloads || 0}</span>
        <button type="button" class="secondary" style="width:auto">Files</button>
      `;
      row.querySelector("button").addEventListener("click", () => showHFFiles(r.id));
      body.appendChild(row);
    });
  } catch (e) {
    if (e.unauthorized) { closeModal("hf-modal"); showLogin(); return; }
    body.innerHTML = '<p class="modal-empty">Search failed: ' + escapeHTML(e.message) + "</p>";
  }
}

async function showHFFiles(repoId) {
  const body = document.getElementById("hf-body");
  body.innerHTML = '<p class="modal-loading">Loading files…</p>';
  try {
    const files = await apiJSON("/api/models/files?repo=" + encodeURIComponent(repoId));
    body.innerHTML = "";
    const back = document.createElement("button");
    back.type = "button";
    back.className = "link-btn";
    back.style.margin = "10px 0 4px 12px";
    back.textContent = "← Back to search results";
    back.addEventListener("click", runHFSearch);
    body.appendChild(back);
    if (!files || !files.length) {
      const p = document.createElement("p");
      p.className = "modal-empty";
      p.textContent = "No .gguf files found in this repo.";
      body.appendChild(p);
      return;
    }
    files.forEach((f) => {
      const row = document.createElement("div");
      row.className = "hf-result";
      row.innerHTML = `
        <span class="hf-name">${escapeHTML(f.filename)}</span>
        <span class="hf-meta">${formatBytes(f.size)}</span>
        <button type="button" style="width:auto">Download</button>
      `;
      row.querySelector("button").addEventListener("click", async () => {
        try {
          await apiJSON("/api/models/download", {
            method: "POST",
            body: JSON.stringify({ repo_id: repoId, filename: f.filename, expected_size: f.size }),
          });
          startDownloadPolling();
        } catch (e) {
          if (e.unauthorized) { closeModal("hf-modal"); showLogin(); return; }
          alert("Could not start download: " + e.message);
        }
      });
      body.appendChild(row);
    });
  } catch (e) {
    if (e.unauthorized) { closeModal("hf-modal"); showLogin(); return; }
    body.innerHTML = '<p class="modal-empty">Could not load files: ' + escapeHTML(e.message) + "</p>";
  }
}

function startDownloadPolling() {
  if (downloadPollTimer) return;
  downloadPollTimer = setInterval(async () => {
    try {
      const progress = await apiJSON("/api/models/download/progress");
      renderDownloadProgress(progress || []);
      if (!progress || !progress.some((p) => p.active)) {
        clearInterval(downloadPollTimer);
        downloadPollTimer = null;
        loadModels();
      }
    } catch (e) {
      clearInterval(downloadPollTimer);
      downloadPollTimer = null;
    }
  }, 1200);
}

function renderDownloadProgress(items) {
  const existing = document.getElementById("hf-progress-list");
  if (existing) existing.remove();
  const active = items.filter((p) => p.active);
  if (!active.length) return;
  const modal = document.getElementById("hf-modal");
  if (modal.classList.contains("hidden")) return;
  const wrap = document.createElement("div");
  wrap.id = "hf-progress-list";
  wrap.style.padding = "10px 20px 0";
  active.forEach((p) => {
    const item = document.createElement("div");
    item.style.marginBottom = "8px";
    item.innerHTML = `
      <div class="status-line" style="margin:0">${escapeHTML(p.filename)} — ${Math.round(p.percent)}%${p.speed ? " · " + escapeHTML(p.speed) : ""}</div>
      <div class="progress-bar"><div style="width:${p.percent}%"></div></div>
    `;
    wrap.appendChild(item);
  });
  document.querySelector("#hf-modal .modal-header").insertAdjacentElement("afterend", wrap);
}

// ── settings: providers ─────────────────────────────────────────────────

let editingProvider = null; // {type, name} while the form is editing an existing provider, else null

async function loadProviders() {
  const list = document.getElementById("provider-list");
  try {
    const providers = await apiJSON("/api/providers");
    const active = await apiJSON("/api/providers/active");
    list.innerHTML = "";
    if (!providers || !providers.length) {
      list.innerHTML = '<p class="status-line">No providers configured yet — add one below.</p>';
      return;
    }
    providers.forEach((p) => {
      const row = document.createElement("div");
      row.className = "row";
      const isActive = active && active.provider === p.name;
      row.innerHTML = `
        <span class="dot ${p.connected ? "on" : ""}"></span>
        <span class="name">${escapeHTML(p.name)}</span>
        <span class="type-tag">${escapeHTML(p.type)}</span>
        ${isActive ? '<span class="active-badge">Active</span>' : '<button type="button" class="secondary set-active" style="width:auto">Use this</button>'}
        <span class="row-actions">
          <button type="button" class="icon-btn edit-provider" title="Edit" aria-label="Edit ${escapeHTML(p.name)}">✎</button>
          <button type="button" class="icon-btn delete-provider" title="Delete" aria-label="Delete ${escapeHTML(p.name)}">🗑</button>
        </span>
      `;
      if (!isActive) {
        row.querySelector(".set-active").addEventListener("click", async () => {
          try {
            await apiJSON("/api/providers/active", { method: "PUT", body: JSON.stringify({ provider: p.name }) });
            await loadProviders();
          } catch (e) {
            if (e.unauthorized) showLogin();
          }
        });
      }
      row.querySelector(".edit-provider").addEventListener("click", () => startEditProvider(p));
      row.querySelector(".delete-provider").addEventListener("click", () => deleteProvider(p));
      list.appendChild(row);
    });
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    list.innerHTML = '<p class="status-line">Could not load providers: ' + escapeHTML(e.message) + "</p>";
  }
}

function startEditProvider(p) {
  editingProvider = { type: p.type, name: p.name };
  document.getElementById("p-type").value = p.type;
  document.getElementById("p-name").value = p.name;
  document.getElementById("p-apikey").value = p.api_key || "";
  document.getElementById("p-baseurl").value = p.base_url || "";
  document.getElementById("p-model").value = p.model || "";
  document.getElementById("provider-form-title").textContent = "Editing: " + p.name;
  document.getElementById("provider-cancel-edit").classList.remove("hidden");
  document.getElementById("provider-form").classList.add("editing");
  document.getElementById("provider-form-msg").textContent = "";
  document.getElementById("provider-form").scrollIntoView({ behavior: "smooth", block: "center" });
}

function cancelProviderEdit() {
  editingProvider = null;
  document.getElementById("provider-form").reset();
  document.getElementById("provider-form-title").textContent = "Add a provider";
  document.getElementById("provider-cancel-edit").classList.add("hidden");
  document.getElementById("provider-form").classList.remove("editing");
  document.getElementById("provider-form-msg").textContent = "";
}

async function deleteProvider(p) {
  if (!confirm(`Remove the provider "${p.name}"? This can't be undone.`)) return;
  try {
    await apiJSON("/api/providers", { method: "DELETE", body: JSON.stringify({ type: p.type, name: p.name }) });
    if (editingProvider && editingProvider.name === p.name) cancelProviderEdit();
    await loadProviders();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    alert("Could not remove provider: " + e.message);
  }
}

function providerFormValue() {
  return {
    type: document.getElementById("p-type").value,
    name: document.getElementById("p-name").value.trim(),
    model: document.getElementById("p-model").value.trim(),
    api_key: document.getElementById("p-apikey").value,
    base_url: document.getElementById("p-baseurl").value.trim(),
  };
}

async function fetchModels() {
  const msg = document.getElementById("fetch-models-msg");
  const type = document.getElementById("p-type").value;
  const apiKey = document.getElementById("p-apikey").value;
  const baseUrl = document.getElementById("p-baseurl").value.trim();
  if (!apiKey && type !== "ollama" && type !== "llama.cpp") {
    msg.textContent = "Enter an API key first.";
    return;
  }
  msg.textContent = "Fetching…";
  try {
    const result = await apiJSON("/api/providers/models", {
      method: "POST",
      body: JSON.stringify({ type, api_key: apiKey, base_url: baseUrl }),
    });
    if (result.status !== "ok" || !result.models || !result.models.length) {
      msg.textContent = "No models returned: " + (result.error || "empty list");
      return;
    }
    const list = document.getElementById("p-model-list");
    list.innerHTML = "";
    result.models.forEach((m) => {
      const opt = document.createElement("option");
      opt.value = m;
      list.appendChild(opt);
    });
    msg.textContent = `${result.models.length} models loaded — pick one from the Model field.`;
    const modelInput = document.getElementById("p-model");
    if (!modelInput.value) modelInput.value = result.models[0];
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    msg.textContent = "Failed: " + e.message;
  }
}

async function testProvider() {
  const msg = document.getElementById("provider-form-msg");
  const v = providerFormValue();
  msg.className = "";
  msg.textContent = "Testing…";
  try {
    const result = await apiJSON("/api/providers/test", { method: "POST", body: JSON.stringify(v) });
    msg.className = result.connected ? "success" : "error";
    msg.textContent = result.connected ? "✓ Connected." : "✗ " + (result.error || "Could not connect.");
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    msg.className = "error";
    msg.textContent = "✗ " + e.message;
  }
}

async function saveProvider(ev) {
  ev.preventDefault();
  const msg = document.getElementById("provider-form-msg");
  const v = providerFormValue();
  if (!v.name) { msg.className = "error"; msg.textContent = "Name is required."; return; }
  msg.className = "";
  msg.textContent = "Saving…";
  try {
    await apiJSON("/api/providers", {
      method: "PUT",
      body: JSON.stringify(Object.assign({}, v, { enabled: true, priority: 0 })),
    });
    await apiJSON("/api/providers/active", { method: "PUT", body: JSON.stringify({ provider: v.name }) });
    msg.className = "success";
    msg.textContent = "Saved and activated.";
    cancelProviderEdit();
    await loadProviders();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    msg.className = "error";
    msg.textContent = "Failed: " + e.message;
  }
}

// ── settings: remote access (Faz 2/3, yapacam.md) ──────────────────────
//
// Deliberately narrow, matching this UI's overall scope: status + a
// restart button, not device/auth-mode management (that's the desktop
// app's Settings → Remote Access tab, or `memo remote` over SSH — both
// already cover it, with real validation this page would just have to
// duplicate).

async function loadRemoteAccess() {
  const statusEl = document.getElementById("remote-status");
  const warnEl = document.getElementById("remote-warning");
  try {
    const status = await apiJSON("/api/remote-access");
    const parts = [status.enabled ? "Enabled" : "Disabled"];
    if (status.auth_mode) parts.push("auth: " + status.auth_mode);
    parts.push(status.running ? "running" : "not running");
    statusEl.textContent = parts.join(" · ");
    if (status.auth_warning) {
      warnEl.textContent = "⚠️ " + status.auth_warning;
      warnEl.classList.remove("hidden");
    } else {
      warnEl.classList.add("hidden");
    }
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    statusEl.textContent = "Could not load remote-access status: " + e.message;
  }
}

async function restartBackend() {
  const msg = document.getElementById("restart-msg");
  if (!confirm("Restart the Memo backend now? Anyone connected will be briefly disconnected.")) return;
  msg.textContent = "Restarting…";
  try {
    await api("/api/shutdown", { method: "POST" });
  } catch {
    // The backend closing the connection as it exits looks identical to a
    // network error from here — expected, not a failure (see
    // internal/replcli.Client.Shutdown's identical reasoning).
  }
  msg.textContent = "Shutdown requested. If this backend runs under systemd (`memo service install`) or a container restart policy, it should come back up shortly — otherwise it needs to be started again manually.";
}

// ── settings: accounts (Faz 5.1, yapacam.md) ────────────────────────────
//
// Deliberately no client-side "am I admin" check before showing the
// add/remove controls — this UI has no way to know its own session's role
// without a dedicated endpoint, and every other panel here (Remote
// Access, Providers) already follows the same pattern of just letting the
// backend's own admin-only gating (Server.callerIsAdmin) reject a
// non-admin's attempt with a clear inline error rather than pre-emptively
// hiding the form. GET /api/accounts itself is readable by any
// authenticated caller (matches the device list), so listing always
// works; only add/remove can 403.

async function loadAccounts() {
  const list = document.getElementById("account-list");
  try {
    const accounts = await apiJSON("/api/accounts");
    list.innerHTML = "";
    if (!accounts || !accounts.length) {
      list.innerHTML = '<p class="status-line">No accounts yet.</p>';
      return;
    }
    accounts.forEach((acc) => {
      const row = document.createElement("div");
      row.className = "row";
      row.innerHTML = `
        <span class="name">${escapeHTML(acc.username)}</span>
        <span class="role-badge ${acc.role === "admin" ? "admin" : ""}">${escapeHTML(acc.role)}</span>
        <span class="row-actions"><button type="button" class="icon-btn remove-btn" title="Remove" aria-label="Remove ${escapeHTML(acc.username)}">🗑</button></span>
      `;
      row.querySelector(".remove-btn").addEventListener("click", () => deleteAccount(acc.id, acc.username));
      list.appendChild(row);
    });
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    list.innerHTML = '<p class="status-line">Could not load accounts: ' + escapeHTML(e.message) + "</p>";
  }
}

async function saveAccount(ev) {
  ev.preventDefault();
  const msg = document.getElementById("account-form-msg");
  const username = document.getElementById("acc-username").value.trim();
  const password = document.getElementById("acc-password").value;
  const role = document.getElementById("acc-role").value;
  if (!username || !password) { msg.className = "error"; msg.textContent = "Username and password are required."; return; }
  msg.className = "";
  msg.textContent = "Saving…";
  try {
    await apiJSON("/api/accounts", { method: "POST", body: JSON.stringify({ username, password, role }) });
    msg.className = "success";
    msg.textContent = "Account added.";
    document.getElementById("account-form").reset();
    await loadAccounts();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    msg.className = "error";
    msg.textContent = "Failed: " + e.message;
  }
}

async function deleteAccount(id, username) {
  const msg = document.getElementById("account-form-msg");
  if (!confirm(`Remove the account "${username}"? This can't be undone.`)) return;
  try {
    await apiJSON("/api/accounts/" + encodeURIComponent(id), { method: "DELETE" });
    await loadAccounts();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    msg.className = "error";
    msg.textContent = "Could not remove account: " + e.message;
  }
}

function escapeHTML(s) {
  const d = document.createElement("div");
  d.textContent = String(s == null ? "" : s);
  return d.innerHTML;
}

boot();
