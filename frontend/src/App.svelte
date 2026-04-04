<script>
  import { marked } from 'marked';
  import { onMount, tick } from 'svelte';
  import {
    SendMessage, SendMessageWithImage, SendMessageWithFile,
    NewChat, ListChats, SwitchChat, DeleteChat,
    GetActiveMessages, GetActiveChatID,
    SelectImage, SelectFile,
    CheckConnection, GetMemoryCount,
    GetSystemPrompt, SetSystemPrompt, ResetSystemPrompt,
    ClearAllMemory, ListMemoryFiles, DeleteMemoryFile
  } from '../wailsjs/go/main/App.js';

  marked.setOptions({ breaks: true, gfm: true });

  // State
  let messages = [];
  let input = '';
  let loading = false;
  let chatEl;
  let chats = [];
  let activeChatId = '';
  let sidebarOpen = true;
  let connStatus = { connected: false, models: [] };
  let memCount = 0;

  // Attachments
  let attachedImage = '';
  let attachedFile = '';
  let attachedFileName = '';

  // Settings
  let settingsOpen = false;
  let settingsTab = 'prompt'; // 'prompt' | 'memory'
  let sysPrompt = '';
  let sysPromptSaved = false;
  let memFiles = [];
  let memBusy = false;

  onMount(async () => {
    await refreshChats();
    await loadMessages();
    await refreshStatus();
    setInterval(refreshStatus, 30000);
  });

  async function refreshStatus() {
    try {
      const [s, c] = await Promise.all([CheckConnection(), GetMemoryCount()]);
      connStatus = s;
      memCount = c;
    } catch(e) {}
  }

  async function refreshChats() {
    try {
      chats = await ListChats() || [];
      activeChatId = await GetActiveChatID();
    } catch(e) {}
  }

  async function loadMessages() {
    try {
      const msgs = await GetActiveMessages();
      messages = (msgs || []).map((m, i) => ({
        id: i,
        role: m.role,
        text: m.content,
        image: m.image_path || '',
        file: m.file_path || '',
        time: m.timestamp || ''
      }));
    } catch(e) {
      messages = [];
    }
    await tick();
    scrollBottom();
  }

  function scrollBottom() {
    if (chatEl) chatEl.scrollTop = chatEl.scrollHeight;
  }

  // ─── Send ──────────────────────────

  async function send() {
    const msg = input.trim();
    if ((!msg && !attachedImage && !attachedFile) || loading) return;

    const userText = msg || '(attached file)';
    input = '';
    loading = true;

    const userMsg = { id: Date.now(), role: 'user', text: userText, image: attachedImage, file: attachedFileName, time: now() };
    messages = [...messages, userMsg];
    await tick();
    scrollBottom();

    let reply = '';
    try {
      if (attachedImage) {
        reply = await SendMessageWithImage(userText, attachedImage);
      } else if (attachedFile) {
        reply = await SendMessageWithFile(userText, attachedFile);
      } else {
        reply = await SendMessage(userText);
      }
    } catch (e) {
      reply = '⚠️ ' + (e?.message || e);
    }

    attachedImage = '';
    attachedFile = '';
    attachedFileName = '';

    messages = [...messages, { id: Date.now() + 1, role: 'assistant', text: reply, image: '', file: '', time: now() }];
    loading = false;
    await tick();
    scrollBottom();
    refreshChats();
  }

  function now() {
    return new Date().toLocaleTimeString('tr-TR', { hour:'2-digit', minute:'2-digit' });
  }

  function onKey(e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); }
  }

  // ─── Attachments ───────────────────

  async function pickImage() {
    try {
      const path = await SelectImage();
      if (path) { attachedImage = path; attachedFile = ''; attachedFileName = ''; }
    } catch(e) {}
  }

  async function pickFile() {
    try {
      const path = await SelectFile();
      if (path) { attachedFile = path; attachedFileName = path.split('/').pop(); attachedImage = ''; }
    } catch(e) {}
  }

  function clearAttachment() {
    attachedImage = '';
    attachedFile = '';
    attachedFileName = '';
  }

  // ─── Sessions ──────────────────────

  async function createChat() {
    try {
      await NewChat();
      await refreshChats();
      await loadMessages();
    } catch(e) {}
  }

  async function switchTo(id) {
    if (id === activeChatId) return;
    try {
      await SwitchChat(id);
      activeChatId = id;
      await loadMessages();
    } catch(e) {}
  }

  async function deleteChat(e, id) {
    e.stopPropagation();
    try {
      await DeleteChat(id);
      await refreshChats();
      await loadMessages();
    } catch(e) {}
  }

  // ─── Settings ──────────────────────

  async function openSettings() {
    settingsOpen = true;
    settingsTab = 'prompt';
    sysPromptSaved = false;
    try {
      sysPrompt = await GetSystemPrompt() || '';
    } catch(e) { sysPrompt = ''; }
  }

  async function savePrompt() {
    try {
      await SetSystemPrompt(sysPrompt);
      sysPromptSaved = true;
      setTimeout(() => sysPromptSaved = false, 2000);
    } catch(e) {}
  }

  async function resetPrompt() {
    try {
      await ResetSystemPrompt();
      sysPrompt = '';
      sysPromptSaved = true;
      setTimeout(() => sysPromptSaved = false, 2000);
    } catch(e) {}
  }

  async function openMemoryTab() {
    settingsTab = 'memory';
    memBusy = true;
    try {
      memFiles = await ListMemoryFiles() || [];
    } catch(e) { memFiles = []; }
    memBusy = false;
  }

  async function clearAll() {
    memBusy = true;
    try {
      await ClearAllMemory();
      memFiles = [];
      memCount = 0;
    } catch(e) {}
    memBusy = false;
  }

  async function deleteFile(path) {
    try {
      await DeleteMemoryFile(path);
      memFiles = memFiles.filter(f => f.path !== path);
      memCount = Math.max(0, memCount - 1);
    } catch(e) {}
  }
</script>

<div class="layout">
  <!-- Sidebar -->
  {#if sidebarOpen}
  <aside class="sidebar">
    <div class="sb-head">
      <span class="brand">Cortex</span>
      <button class="btn-icon" on:click={createChat} title="New Chat">＋</button>
    </div>

    <div class="sb-list">
      {#each chats as c}
        <button
          class="chat-item"
          class:active={c.id === activeChatId}
          on:click={() => switchTo(c.id)}
        >
          <span class="ci-title">{c.title}</span>
          <span class="ci-meta">{c.msg_count} msg</span>
          <button class="ci-del" on:click={(e) => deleteChat(e, c.id)} title="Delete">×</button>
        </button>
      {/each}
    </div>

    <div class="sb-foot">
      <div class="conn" class:on={connStatus.connected}>
        <span class="dot"></span>
        <span>{connStatus.connected ? (connStatus.models?.[0] || 'Connected') : 'Offline'}</span>
      </div>
      <div class="mem">🧠 {memCount}</div>
    </div>
  </aside>
  {/if}

  <!-- Main -->
  <main class="main">
    <!-- Header -->
    <div class="header">
      <button class="btn-icon" on:click={() => sidebarOpen = !sidebarOpen}>☰</button>
      <span class="header-title">
        {chats.find(c => c.id === activeChatId)?.title || 'Cortex'}
      </span>
      <button class="btn-icon" on:click={openSettings} title="Settings">⚙</button>
    </div>

    <!-- Messages -->
    <div class="chat" bind:this={chatEl}>
      {#if messages.length === 0}
        <div class="empty">
          <div class="empty-icon">◈</div>
          <div class="empty-title">Cortex</div>
          <div class="empty-sub">Yerel AI · Kalıcı Hafıza · Tamamen Özel</div>
        </div>
      {/if}

      {#each messages as m (m.id)}
        <div class="msg {m.role}" style="animation: fadeIn 150ms ease-out">
          <div class="msg-label">{m.role === 'user' ? 'Sen' : 'Cortex'}<span class="msg-time">{m.time}</span></div>
          {#if m.image}
            <div class="msg-attach">📷 {m.image.split('/').pop()}</div>
          {/if}
          {#if m.file}
            <div class="msg-attach">📎 {m.file}</div>
          {/if}
          <div class="bubble {m.role}">
            {#if m.role === 'assistant'}
              <div class="md">{@html marked.parse(m.text || '')}</div>
            {:else}
              <p>{m.text}</p>
            {/if}
          </div>
        </div>
      {/each}

      {#if loading}
        <div class="msg assistant" style="animation: fadeIn 150ms ease-out">
          <div class="msg-label">Cortex</div>
          <div class="bubble assistant thinking">Düşünüyor<span class="dots">...</span></div>
        </div>
      {/if}
    </div>

    <!-- Attachment preview -->
    {#if attachedImage || attachedFile}
      <div class="attach-bar">
        {#if attachedImage}
          <span>📷 {attachedImage.split('/').pop()}</span>
        {:else}
          <span>📎 {attachedFileName}</span>
        {/if}
        <button class="attach-clear" on:click={clearAttachment}>×</button>
      </div>
    {/if}

    <!-- Input -->
    <div class="input-area">
      <div class="input-box">
        <button class="btn-attach" on:click={pickImage} disabled={loading} title="Send Image">📷</button>
        <button class="btn-attach" on:click={pickFile} disabled={loading} title="Send File">📎</button>
        <textarea
          bind:value={input}
          on:keydown={onKey}
          placeholder="Mesajını yaz..."
          rows="1"
          disabled={loading}
        ></textarea>
        <button class="btn-send" on:click={send} disabled={(!input.trim() && !attachedImage && !attachedFile) || loading}>
          {#if loading}
            <span class="spinner"></span>
          {:else}
            →
          {/if}
        </button>
      </div>
    </div>
  </main>
</div>

<!-- Settings Modal -->
{#if settingsOpen}
<div class="modal-overlay" on:click={() => settingsOpen = false}>
  <div class="modal" on:click|stopPropagation>
    <div class="modal-head">
      <h2>⚙ Ayarlar</h2>
      <button class="btn-icon modal-close" on:click={() => settingsOpen = false}>×</button>
    </div>

    <div class="modal-tabs">
      <button class="tab" class:active={settingsTab === 'prompt'} on:click={() => settingsTab = 'prompt'}>
        Sistem Prompt
      </button>
      <button class="tab" class:active={settingsTab === 'memory'} on:click={openMemoryTab}>
        Hafıza Yönetimi
      </button>
    </div>

    <div class="modal-body">
      {#if settingsTab === 'prompt'}
        <div class="setting-section">
          <p class="setting-desc">
            Asistanın nasıl davranacağını buradan belirle. Boş bırakırsan varsayılan prompt kullanılır.
          </p>
          <textarea
            class="prompt-input"
            bind:value={sysPrompt}
            placeholder="Örn: Sen bir yazılım uzmanısın. Soruları Türkçe cevapla..."
            rows="8"
          ></textarea>
          <div class="setting-actions">
            <button class="btn-primary" on:click={savePrompt}>Kaydet</button>
            <button class="btn-secondary" on:click={resetPrompt}>Varsayılana Dön</button>
            {#if sysPromptSaved}
              <span class="save-ok">✓ Kaydedildi</span>
            {/if}
          </div>
        </div>

      {:else if settingsTab === 'memory'}
        <div class="setting-section">
          <div class="mem-header">
            <p class="setting-desc">
              Toplam hafıza: <strong>{memFiles.length} kayıt</strong>
            </p>
            <button class="btn-danger" on:click={clearAll} disabled={memBusy || memFiles.length === 0}>
              {memBusy ? '...' : '🗑 Tümünü Sil'}
            </button>
          </div>

          {#if memBusy}
            <div class="mem-loading">Yükleniyor...</div>
          {:else if memFiles.length === 0}
            <div class="mem-empty">Hafıza boş — henüz kayıt yok.</div>
          {:else}
            <div class="mem-list">
              {#each memFiles as f}
                <div class="mem-item">
                  <div class="mem-info">
                    <span class="mem-name">{f.name}</span>
                    <span class="mem-meta">{f.size_kb} KB · {f.modified}</span>
                  </div>
                  <button class="mem-del" on:click={() => deleteFile(f.path)} title="Sil">×</button>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>
{/if}

<style>
  .layout {
    height: 100vh;
    display: flex;
    overflow: hidden;
  }

  /* ─── Sidebar ─── */
  .sidebar {
    width: 260px;
    background: var(--bg-1);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
  }
  .sb-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
  }
  .brand {
    font-size: 17px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: -0.3px;
  }
  .sb-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }
  .chat-item {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 10px 12px;
    border-radius: var(--r-md);
    text-align: left;
    color: var(--text-1);
    font-size: 13px;
    margin-bottom: 2px;
    position: relative;
  }
  .chat-item:hover { background: var(--bg-hover); color: var(--text-0); }
  .chat-item.active {
    background: var(--accent-soft);
    color: var(--accent);
    border: 1px solid var(--border-accent);
  }
  .ci-title { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ci-meta { font-size: 10px; color: var(--text-2); flex-shrink: 0; }
  .ci-del {
    opacity: 0;
    font-size: 16px;
    color: var(--text-2);
    width: 20px; height: 20px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px;
    flex-shrink: 0;
  }
  .chat-item:hover .ci-del { opacity: 1; }
  .ci-del:hover { background: var(--bg-active); color: var(--red); }
  .sb-foot {
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 11px;
    color: var(--text-2);
  }
  .conn { display: flex; align-items: center; gap: 6px; }
  .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--red); }
  .conn.on .dot { background: var(--green); box-shadow: 0 0 6px var(--green); }
  .mem { color: var(--text-2); }

  /* ─── Main ─── */
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-0);
  }
  .header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
    flex-shrink: 0;
  }
  .header-title {
    flex: 1;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-0);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ─── Chat ─── */
  .chat { flex: 1; overflow-y: auto; padding: 20px 24px; }
  .empty {
    display: flex; flex-direction: column; align-items: center; justify-content: center;
    height: 100%; gap: 8px;
  }
  .empty-icon { font-size: 48px; color: var(--accent); opacity: 0.6; }
  .empty-title { font-size: 26px; font-weight: 700; color: var(--text-0); letter-spacing: -0.5px; }
  .empty-sub { font-size: 13px; color: var(--text-2); }

  .msg {
    margin-bottom: 16px;
    max-width: 720px;
    margin-left: auto; margin-right: auto;
    width: 100%;
  }
  .msg.user { text-align: right; }
  .msg-label { font-size: 11px; font-weight: 600; color: var(--text-2); margin-bottom: 4px; }
  .msg-time { font-weight: 400; margin-left: 8px; color: var(--text-3); }
  .msg-attach { font-size: 11px; color: var(--accent); margin-bottom: 4px; }
  .bubble {
    display: inline-block; padding: 10px 16px;
    border-radius: var(--r-lg); font-size: 14px; line-height: 1.65;
    white-space: pre-wrap; word-break: break-word; text-align: left;
  }
  .bubble.user {
    background: var(--accent); color: #0a0a0a;
    border-bottom-right-radius: 4px; font-weight: 450;
  }
  .bubble.assistant {
    background: var(--bg-2); color: var(--text-0);
    border: 1px solid var(--border); border-bottom-left-radius: 4px;
  }
  .bubble p { margin: 0; }
  .thinking { color: var(--text-2); font-style: italic; }
  .dots { animation: fadeIn 1s infinite alternate; }

  /* ─── Attachment ─── */
  .attach-bar {
    display: flex; align-items: center; gap: 8px;
    padding: 6px 24px; font-size: 12px; color: var(--accent);
    background: var(--bg-1); border-top: 1px solid var(--border);
  }
  .attach-clear {
    font-size: 16px; color: var(--text-2);
    width: 20px; height: 20px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px;
  }
  .attach-clear:hover { background: var(--bg-hover); color: var(--red); }

  /* ─── Input ─── */
  .input-area { padding: 0 24px 16px; max-width: 768px; margin: 0 auto; width: 100%; }
  .input-box {
    display: flex; align-items: flex-end; gap: 4px;
    padding: 6px 8px; border-radius: var(--r-xl);
    background: var(--bg-2); border: 1px solid var(--border);
    transition: border-color 150ms;
  }
  .input-box:focus-within {
    border-color: var(--accent-dim);
    box-shadow: 0 0 0 2px var(--accent-glow);
  }
  .btn-attach {
    width: 34px; height: 34px; border-radius: var(--r-sm);
    font-size: 15px; display: flex; align-items: center; justify-content: center;
    color: var(--text-2); flex-shrink: 0;
  }
  .btn-attach:hover:not(:disabled) { background: var(--bg-hover); color: var(--text-0); }
  .input-box textarea {
    flex: 1; background: none; border: none; color: var(--text-0);
    font-size: 14px; line-height: 1.5; resize: none;
    min-height: 22px; max-height: 140px; padding: 6px 4px; outline: none;
  }
  .btn-send {
    width: 38px; height: 38px; border-radius: var(--r-md);
    background: var(--accent); color: #0a0a0a;
    font-size: 18px; font-weight: 700;
    display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  }
  .btn-send:hover:not(:disabled) { background: var(--accent-dim); transform: scale(1.04); }
  .btn-send:disabled { opacity: 0.2; }
  .spinner {
    width: 16px; height: 16px;
    border: 2px solid rgba(0,0,0,0.2);
    border-top-color: #0a0a0a;
    border-radius: 50%;
    animation: spin 0.5s linear infinite;
  }
  .btn-icon {
    width: 32px; height: 32px; border-radius: var(--r-sm);
    font-size: 18px; display: flex; align-items: center; justify-content: center;
    color: var(--text-1);
  }
  .btn-icon:hover { background: var(--bg-hover); color: var(--text-0); }

  /* ─── Settings Modal ─── */
  .modal-overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.7);
    backdrop-filter: blur(4px);
    display: flex; align-items: center; justify-content: center;
    z-index: 1000;
    animation: fadeIn 120ms ease-out;
  }
  .modal {
    width: 560px;
    max-height: 80vh;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    display: flex;
    flex-direction: column;
    box-shadow: 0 16px 48px rgba(0,0,0,0.5);
  }
  .modal-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border);
  }
  .modal-head h2 { font-size: 16px; font-weight: 700; color: var(--text-0); }
  .modal-close { font-size: 22px; }
  .modal-tabs {
    display: flex;
    padding: 0 20px;
    border-bottom: 1px solid var(--border);
  }
  .tab {
    padding: 12px 16px;
    font-size: 13px;
    font-weight: 500;
    color: var(--text-2);
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
  }
  .tab:hover { color: var(--text-0); }
  .tab.active {
    color: var(--accent);
    border-bottom-color: var(--accent);
  }
  .modal-body {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
  }
  .setting-section { }
  .setting-desc {
    font-size: 13px;
    color: var(--text-1);
    margin-bottom: 12px;
    line-height: 1.5;
  }
  .prompt-input {
    width: 100%;
    min-height: 160px;
    background: var(--bg-0);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    color: var(--text-0);
    font-size: 13px;
    padding: 12px;
    line-height: 1.6;
    resize: vertical;
    font-family: var(--mono);
  }
  .prompt-input:focus {
    border-color: var(--accent-dim);
    outline: none;
  }
  .setting-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
  }
  .btn-primary {
    padding: 8px 20px;
    border-radius: var(--r-md);
    background: var(--accent);
    color: #0a0a0a;
    font-weight: 600;
    font-size: 13px;
  }
  .btn-primary:hover { background: var(--accent-dim); }
  .btn-secondary {
    padding: 8px 16px;
    border-radius: var(--r-md);
    background: var(--bg-3);
    color: var(--text-1);
    font-size: 13px;
  }
  .btn-secondary:hover { background: var(--bg-hover); color: var(--text-0); }
  .btn-danger {
    padding: 8px 16px;
    border-radius: var(--r-md);
    background: rgba(248,113,113,0.1);
    color: var(--red);
    font-size: 13px;
    font-weight: 600;
    border: 1px solid rgba(248,113,113,0.2);
  }
  .btn-danger:hover { background: rgba(248,113,113,0.2); }
  .btn-danger:disabled { opacity: 0.3; }
  .save-ok {
    font-size: 12px;
    color: var(--green);
    animation: fadeIn 200ms ease-out;
  }

  /* Memory list */
  .mem-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }
  .mem-header .setting-desc { margin-bottom: 0; }
  .mem-loading, .mem-empty {
    text-align: center;
    padding: 32px;
    color: var(--text-2);
    font-size: 13px;
  }
  .mem-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-height: 320px;
    overflow-y: auto;
  }
  .mem-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-radius: var(--r-sm);
    background: var(--bg-0);
    border: 1px solid var(--border);
  }
  .mem-item:hover { border-color: rgba(248,113,113,0.2); }
  .mem-info { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0; }
  .mem-name {
    font-size: 12px;
    font-family: var(--mono);
    color: var(--text-0);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mem-meta { font-size: 10px; color: var(--text-2); }
  .mem-del {
    font-size: 16px;
    color: var(--text-2);
    width: 24px; height: 24px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px;
    flex-shrink: 0;
    opacity: 0.5;
  }
  .mem-del:hover { background: rgba(248,113,113,0.15); color: var(--red); opacity: 1; }
</style>
