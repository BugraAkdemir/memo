// Memo — minimal web client. No build step, no framework: talks to the same
// REST/SSE API the desktop app and CLI use. Scope is deliberately narrow —
// chat plus the handful of settings you need before the full desktop app
// (which can already manage everything else remotely, see AGENTS.md) is
// reachable: starting/stopping the local model, connecting a provider.

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

function showLogin() {
  document.getElementById("setup-screen").classList.add("hidden");
  document.getElementById("login-screen").classList.remove("hidden");
  document.getElementById("app").classList.add("hidden");

  // Show whichever credential form(s) the configured auth mode actually
  // checks (see internal/webserver's remoteAuthOK) — "password" never
  // checks a device token and vice versa, so offering the other one would
  // just be a confusing dead end. Unknown/empty mode (e.g. the
  // setup-status probe above failed) falls back to the token-only form,
  // matching this page's behavior before password login existed here.
  const showPassword = authMode === "password" || authMode === "token_password";
  const showToken = authMode !== "password";
  document.getElementById("password-login-block").classList.toggle("hidden", !showPassword);
  document.getElementById("token-login-block").classList.toggle("hidden", !showToken);
  document.getElementById("login-divider").classList.toggle("hidden", !(showPassword && showToken));

  if (showPassword) {
    document.getElementById("login-username").focus();
  } else {
    document.getElementById("token-input").focus();
  }
}

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
  if (!username || !password) return;
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
document.getElementById("login-password").addEventListener("keydown", (e) => {
  if (e.key === "Enter") document.getElementById("login-submit").click();
});

document.getElementById("setup-submit").addEventListener("click", async () => {
  const username = document.getElementById("setup-username").value.trim();
  const password = document.getElementById("setup-password").value;
  const confirm = document.getElementById("setup-password-confirm").value;
  const errEl = document.getElementById("setup-error");
  errEl.classList.add("hidden");
  if (!username || !password) {
    errEl.textContent = "Username and password are required.";
    errEl.classList.remove("hidden");
    return;
  }
  if (password !== confirm) {
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
document.getElementById("setup-password-confirm").addEventListener("keydown", (e) => {
  if (e.key === "Enter") document.getElementById("setup-submit").click();
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
  document.getElementById("provider-form").addEventListener("submit", saveProvider);
  document.getElementById("p-test").addEventListener("click", testProvider);
  document.getElementById("p-fetch-models").addEventListener("click", fetchModels);
  document.getElementById("restart-backend").addEventListener("click", restartBackend);
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
  await Promise.all([loadModels(), loadProviders(), loadRemoteAccess()]);
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
      statusEl.textContent = (models && models.length) ? "Not running." : "No local models downloaded yet — use the desktop app's Model Store to download one first.";
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

// ── settings: providers ─────────────────────────────────────────────────

async function loadProviders() {
  const list = document.getElementById("provider-list");
  try {
    const providers = await apiJSON("/api/providers");
    const active = await apiJSON("/api/providers/active");
    list.innerHTML = "";
    if (!providers || !providers.length) {
      list.innerHTML = '<p class="status-line">No providers configured yet.</p>';
      return;
    }
    providers.forEach((p) => {
      const row = document.createElement("div");
      row.className = "provider-row";
      const isActive = active && active.provider === p.name;
      row.innerHTML = `
        <span class="dot ${p.connected ? "on" : ""}"></span>
        <span class="name">${escapeHTML(p.name)}</span>
        <span class="type">${escapeHTML(p.type)}</span>
        ${isActive ? '<span class="active-badge">active</span>' : '<button type="button" class="secondary set-active">Use this</button>'}
      `;
      if (!isActive) {
        row.querySelector(".set-active").addEventListener("click", async () => {
          try {
            await apiJSON("/api/providers/active", {
              method: "PUT",
              body: JSON.stringify({ provider: p.name }),
            });
            await loadProviders();
          } catch (e) {
            if (e.unauthorized) showLogin();
          }
        });
      }
      list.appendChild(row);
    });
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    list.innerHTML = '<p class="status-line">Could not load providers: ' + escapeHTML(e.message) + "</p>";
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
  msg.textContent = "Testing…";
  try {
    const result = await apiJSON("/api/providers/test", {
      method: "POST",
      body: JSON.stringify(v),
    });
    msg.textContent = result.connected ? "✓ Connected." : "✗ " + (result.error || "Could not connect.");
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
    msg.textContent = "✗ " + e.message;
  }
}

async function saveProvider(ev) {
  ev.preventDefault();
  const msg = document.getElementById("provider-form-msg");
  const v = providerFormValue();
  if (!v.name) { msg.textContent = "Name is required."; return; }
  msg.textContent = "Saving…";
  try {
    await apiJSON("/api/providers", {
      method: "PUT",
      body: JSON.stringify(Object.assign({}, v, { enabled: true, priority: 0 })),
    });
    await apiJSON("/api/providers/active", {
      method: "PUT",
      body: JSON.stringify({ provider: v.name }),
    });
    msg.textContent = "Saved and activated.";
    document.getElementById("provider-form").reset();
    await loadProviders();
  } catch (e) {
    if (e.unauthorized) { showLogin(); return; }
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

function escapeHTML(s) {
  const d = document.createElement("div");
  d.textContent = String(s == null ? "" : s);
  return d.innerHTML;
}

boot();
