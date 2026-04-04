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

  let messages = [];
  let input = '';
  let loading = false;
  let chatEl;
  let chats = [];
  let activeChatId = '';
  let sidebarOpen = true;
  let conn = { connected: false, models: [] };
  let memCount = 0;

  let attachedImage = '';
  let attachedFile = '';
  let attachedFileName = '';

  let settingsOpen = false;
  let settingsTab = 'prompt';
  let sysPrompt = '';
  let promptSaved = false;
  let memFiles = [];
  let memBusy = false;

  onMount(async () => {
    await refreshAll();
    setInterval(refreshStatus, 30000);
  });

  async function refreshAll() {
    await refreshChats();
    await loadMessages();
    await refreshStatus();
  }

  async function refreshStatus() {
    try {
      const [s, c] = await Promise.all([CheckConnection(), GetMemoryCount()]);
      conn = s;
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
        id: i, role: m.role, text: m.content,
        image: m.image_path || '', file: m.file_path || '', time: m.timestamp || ''
      }));
    } catch(e) { messages = []; }
    await tick();
    scroll();
  }

  function scroll() { if (chatEl) chatEl.scrollTop = chatEl.scrollHeight; }
  function ts() { return new Date().toLocaleTimeString('tr-TR', { hour:'2-digit', minute:'2-digit' }); }

  // ─── Send ─────────────────────
  async function send() {
    const msg = input.trim();
    if ((!msg && !attachedImage && !attachedFile) || loading) return;
    const text = msg || '(file attached)';
    input = '';
    loading = true;
    messages = [...messages, { id: Date.now(), role: 'user', text, image: attachedImage, file: attachedFileName, time: ts() }];
    await tick(); scroll();

    let reply = '';
    try {
      if (attachedImage) reply = await SendMessageWithImage(text, attachedImage);
      else if (attachedFile) reply = await SendMessageWithFile(text, attachedFile);
      else reply = await SendMessage(text);
    } catch (e) { reply = '⚠ ' + (e?.message || e); }

    attachedImage = ''; attachedFile = ''; attachedFileName = '';
    messages = [...messages, { id: Date.now()+1, role: 'assistant', text: reply, image:'', file:'', time: ts() }];
    loading = false;
    await tick(); scroll();
    refreshChats();
  }

  function onKey(e) { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }

  // ─── Attach ───────────────────
  async function pickImage() {
    try { const p = await SelectImage(); if (p) { attachedImage=p; attachedFile=''; attachedFileName=''; } } catch(e) {}
  }
  async function pickFile() {
    try { const p = await SelectFile(); if (p) { attachedFile=p; attachedFileName=p.split('/').pop(); attachedImage=''; } } catch(e) {}
  }
  function clearAttach() { attachedImage=''; attachedFile=''; attachedFileName=''; }

  // ─── Sessions ─────────────────
  async function newChat() { try { await NewChat(); await refreshChats(); await loadMessages(); } catch(e) {} }
  async function switchTo(id) { if (id===activeChatId) return; try { await SwitchChat(id); activeChatId=id; await loadMessages(); } catch(e) {} }
  async function delChat(ev, id) { ev.stopPropagation(); try { await DeleteChat(id); await refreshChats(); await loadMessages(); } catch(e) {} }

  // ─── Settings ─────────────────
  async function openSettings() {
    settingsOpen = true; settingsTab = 'prompt'; promptSaved = false;
    try { sysPrompt = await GetSystemPrompt() || ''; } catch(e) { sysPrompt=''; }
  }
  async function savePrompt() {
    try { await SetSystemPrompt(sysPrompt); promptSaved=true; setTimeout(()=>promptSaved=false,2000); } catch(e) {}
  }
  async function resetPrompt() {
    try { await ResetSystemPrompt(); sysPrompt=''; promptSaved=true; setTimeout(()=>promptSaved=false,2000); } catch(e) {}
  }
  async function openMemTab() {
    settingsTab='memory'; memBusy=true;
    try { memFiles = await ListMemoryFiles() || []; } catch(e) { memFiles=[]; }
    memBusy=false;
  }
  async function clearMem() {
    memBusy=true;
    try { await ClearAllMemory(); memFiles=[]; memCount=0; } catch(e) {}
    memBusy=false;
  }
  async function delMem(path) {
    try { await DeleteMemoryFile(path); memFiles=memFiles.filter(f=>f.path!==path); memCount=Math.max(0,memCount-1); } catch(e) {}
  }
</script>

<div class="shell">
  <!-- ═══ Sidebar ═══ -->
  {#if sidebarOpen}
  <aside class="side">
    <div class="side-top">
      <button class="new-session" on:click={newChat}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        <span>New Session</span>
      </button>
    </div>

    <nav class="sessions">
      {#each chats as c}
        <button class="s-item" class:active={c.id === activeChatId} on:click={() => switchTo(c.id)}>
          <div class="s-info">
            <span class="s-title">{c.title}</span>
            <span class="s-time">{c.updated_at}</span>
          </div>
          <button class="s-del" on:click={(e) => delChat(e, c.id)}>
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </button>
      {/each}
    </nav>

    <div class="side-bottom">
      <div class="status" class:on={conn.connected}>
        <span class="dot"></span>
        <span class="status-text">{conn.connected ? (conn.models?.[0] || 'online') : 'offline'}</span>
      </div>
      <span class="mem-badge">{memCount} memories</span>
    </div>
  </aside>
  {/if}

  <!-- ═══ Main ═══ -->
  <main class="main">
    <!-- Header -->
    <header class="bar">
      <button class="bar-btn" on:click={() => sidebarOpen = !sidebarOpen}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
      </button>
      <span class="bar-title">{chats.find(c => c.id === activeChatId)?.title || 'Memo'}</span>
      <button class="bar-btn" on:click={openSettings}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
      </button>
    </header>

    <!-- Feed -->
    <div class="feed" bind:this={chatEl}>
      {#if messages.length === 0}
        <div class="welcome">
          <div class="w-mark">memo</div>
          <div class="w-sub">local ai · persistent memory · private</div>
        </div>
      {/if}

      {#each messages as m (m.id)}
        <div class="entry" class:memo={m.role==='assistant'} style="animation:fadeIn 120ms ease-out">
          <div class="entry-head">
            <span class="entry-sender">{m.role === 'user' ? 'Buğra' : 'Memo'} ›</span>
            <span class="entry-time">{m.time}</span>
          </div>
          {#if m.image}
            <div class="entry-attach">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>
              {m.image.split('/').pop()}
            </div>
          {/if}
          {#if m.file}
            <div class="entry-attach">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
              {m.file}
            </div>
          {/if}
          <div class="entry-body">
            {#if m.role === 'assistant'}
              <div class="md">{@html marked.parse(m.text || '')}</div>
            {:else}
              {m.text}
            {/if}
          </div>
        </div>
      {/each}

      {#if loading}
        <div class="entry memo" style="animation:fadeIn 120ms ease-out">
          <div class="entry-head">
            <span class="entry-sender">Memo ›</span>
          </div>
          <div class="entry-body thinking">
            <span class="cursor-blink">▊</span>
          </div>
        </div>
      {/if}
    </div>

    <!-- Attachment preview -->
    {#if attachedImage || attachedFile}
      <div class="attach-row">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--gold)" stroke-width="1.5"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
        <span>{attachedImage ? attachedImage.split('/').pop() : attachedFileName}</span>
        <button class="attach-x" on:click={clearAttach}>×</button>
      </div>
    {/if}

    <!-- Input -->
    <div class="input-dock">
      <div class="input-row">
        <button class="dock-btn" on:click={pickImage} disabled={loading} title="Attach image">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>
        </button>
        <button class="dock-btn" on:click={pickFile} disabled={loading} title="Attach file">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
        </button>
        <textarea
          bind:value={input}
          on:keydown={onKey}
          placeholder="Type a message to Memo..."
          rows="1"
          disabled={loading}
        ></textarea>
        <button class="send-btn" on:click={send} disabled={(!input.trim() && !attachedImage && !attachedFile) || loading}>
          {#if loading}
            <span class="spin"></span>
          {:else}
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
          {/if}
        </button>
      </div>
    </div>
  </main>
</div>

<!-- ═══ Settings Modal ═══ -->
{#if settingsOpen}
<div class="overlay" on:click={() => settingsOpen=false}>
  <div class="modal" on:click|stopPropagation>
    <div class="m-head">
      <span class="m-title">Settings</span>
      <button class="bar-btn" on:click={() => settingsOpen=false}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
    <div class="m-tabs">
      <button class="m-tab" class:active={settingsTab==='prompt'} on:click={() => settingsTab='prompt'}>System Prompt</button>
      <button class="m-tab" class:active={settingsTab==='memory'} on:click={openMemTab}>Memory</button>
    </div>
    <div class="m-body">
      {#if settingsTab === 'prompt'}
        <p class="m-desc">Define how Memo behaves. Leave empty for default.</p>
        <textarea class="m-prompt" bind:value={sysPrompt} placeholder="e.g. You are a senior Go developer..." rows="8"></textarea>
        <div class="m-actions">
          <button class="m-btn gold" on:click={savePrompt}>Save</button>
          <button class="m-btn" on:click={resetPrompt}>Reset Default</button>
          {#if promptSaved}<span class="m-ok">✓ saved</span>{/if}
        </div>
      {:else}
        <div class="mem-top">
          <p class="m-desc">{memFiles.length} memory records stored</p>
          <button class="m-btn danger" on:click={clearMem} disabled={memBusy || !memFiles.length}>
            {memBusy ? '...' : 'Clear All'}
          </button>
        </div>
        {#if memBusy}
          <div class="mem-empty">Loading...</div>
        {:else if !memFiles.length}
          <div class="mem-empty">No memories stored yet.</div>
        {:else}
          <div class="mem-grid">
            {#each memFiles as f}
              <div class="mem-row">
                <div class="mem-info">
                  <span class="mem-name">{f.name}</span>
                  <span class="mem-meta">{f.size_kb}KB · {f.modified}</span>
                </div>
                <button class="mem-x" on:click={() => delMem(f.path)}>×</button>
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </div>
  </div>
</div>
{/if}

<style>
  /* ═══════════════════════════════════
     SHELL LAYOUT
  ═══════════════════════════════════ */
  .shell { height:100vh; display:flex; overflow:hidden; }

  /* ─── SIDEBAR ─── */
  .side {
    width: 240px;
    background: var(--black-1);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    border-right: 1px solid var(--t-3);
  }
  .side-top { padding: var(--sp-4); }
  .new-session {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    width: 100%;
    padding: var(--sp-3) var(--sp-4);
    color: var(--gold);
    font-size: 14px;
    font-weight: 500;
    letter-spacing: 0.3px;
    border-radius: var(--r);
  }
  .new-session:hover { background: var(--gold-subtle); }

  .sessions { flex:1; overflow-y:auto; padding: 0 var(--sp-2); }
  .s-item {
    display: flex;
    align-items: center;
    width: 100%;
    padding: var(--sp-3) var(--sp-3);
    border-bottom: 1px solid var(--t-3);
    text-align: left;
    color: var(--t-1);
    font-size: 14px;
    gap: var(--sp-2);
  }
  .s-item:last-child { border-bottom: none; }
  .s-item:hover { color: var(--t-0); background: var(--black-2); }
  .s-item.active { color: var(--gold); background: var(--gold-subtle); }
  .s-info { flex:1; min-width:0; display:flex; flex-direction:column; gap:1px; }
  .s-title {
    font-size: 14px;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .s-time { font-size: 11px; color: var(--t-2); }
  .s-del {
    opacity: 0;
    width: 24px; height: 24px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px;
    color: var(--t-2);
    flex-shrink: 0;
  }
  .s-item:hover .s-del { opacity: 1; }
  .s-del:hover { background: var(--black-4); color: var(--red); }

  .side-bottom {
    padding: var(--sp-3) var(--sp-4);
    border-top: 1px solid var(--t-3);
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 10px;
    color: var(--t-2);
  }
  .status { display:flex; align-items:center; gap: 6px; }
  .dot { width:5px; height:5px; border-radius:50%; background:var(--red); }
  .status.on .dot { background: var(--green); }
  .status-text { letter-spacing: 0.3px; font-size: 12px; }
  .mem-badge { color: var(--t-2); font-size: 12px; }

  /* ─── MAIN AREA ─── */
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--black-0);
  }

  .bar {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-2) var(--sp-4);
    border-bottom: 1px solid var(--t-3);
    background: var(--black-1);
    flex-shrink: 0;
    height: 44px;
  }
  .bar-title {
    flex: 1;
    font-size: 14px;
    font-weight: 600;
    color: var(--t-1);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    letter-spacing: 0.2px;
  }
  .bar-btn {
    width: 36px; height: 36px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--r);
    color: var(--t-2);
  }
  .bar-btn:hover { background: var(--black-4); color: var(--t-0); }

  /* ─── FEED ─── */
  .feed {
    flex: 1;
    overflow-y: auto;
    padding: var(--sp-5) var(--sp-6);
  }

  .welcome {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: var(--sp-2);
  }
  .w-mark {
    font-family: var(--mono);
    font-size: 36px;
    font-weight: 500;
    color: var(--gold);
    letter-spacing: -1px;
    opacity: 0.7;
  }
  .w-sub {
    font-size: 11px;
    color: var(--t-2);
    letter-spacing: 1px;
    text-transform: uppercase;
  }

  .entry {
    max-width: 90%;
    margin: 0 auto var(--sp-4);
    padding: var(--sp-3) var(--sp-4);
    border-radius: var(--r);
    width: 100%;
  }
  .entry.memo {
    background: var(--black-2);
  }
  .entry-head {
    display: flex;
    align-items: baseline;
    gap: var(--sp-2);
    margin-bottom: var(--sp-1);
  }
  .entry-sender {
    font-weight: 700;
    font-size: 14px;
    color: var(--gold);
    font-family: var(--mono);
  }
  .entry-time {
    font-size: 12px;
    color: var(--t-3);
  }
  .entry-attach {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: 13px;
    color: var(--t-2);
    margin-bottom: var(--sp-1);
  }
  .entry-body {
    font-size: 15px;
    line-height: 1.7;
    color: var(--t-0);
  }
  .thinking { color: var(--t-2); }
  .cursor-blink { animation: pulse 1s infinite; color: var(--gold); }

  /* ─── ATTACHMENT ROW ─── */
  .attach-row {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-1) var(--sp-6);
    font-size: 11px;
    color: var(--gold);
  }
  .attach-x {
    font-size: 14px;
    color: var(--t-2);
    width: 18px; height: 18px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 3px;
  }
  .attach-x:hover { color: var(--red); }

  /* ─── INPUT DOCK ─── */
  .input-dock {
    padding: var(--sp-2) var(--sp-5);
    background: rgba(5,5,5,0.85);
    backdrop-filter: blur(12px);
    border-top: 1px solid var(--t-3);
  }
  .input-row {
    display: flex;
    align-items: flex-end;
    gap: var(--sp-1);
    max-width: 90%;
    margin: 0 auto;
  }
  .dock-btn {
    width: 38px; height: 38px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--r);
    color: var(--t-2);
    flex-shrink: 0;
  }
  .dock-btn:hover:not(:disabled) { color: var(--gold); background: var(--black-3); }
  .input-row textarea {
    flex: 1;
    background: transparent;
    border: none;
    color: var(--t-0);
    font-size: 15px;
    line-height: 1.5;
    resize: none;
    min-height: 20px;
    max-height: 120px;
    padding: var(--sp-2) var(--sp-2);
    outline: none;
    font-family: var(--sans);
  }
  .input-row textarea::placeholder { color: var(--gold-muted); }
  .input-row textarea:disabled { opacity: 0.35; }
  .send-btn {
    width: 38px; height: 38px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--r);
    color: var(--gold);
    flex-shrink: 0;
  }
  .send-btn:hover:not(:disabled) { background: var(--gold-subtle); }
  .send-btn:disabled { color: var(--t-3); }
  .spin {
    width: 14px; height: 14px;
    border: 1.5px solid var(--t-3);
    border-top-color: var(--gold);
    border-radius: 50%;
    animation: spin 0.5s linear infinite;
  }

  /* ═══ SETTINGS MODAL ═══ */
  .overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.75);
    backdrop-filter: blur(4px);
    display: flex; align-items: center; justify-content: center;
    z-index: 100;
    animation: fadeIn 100ms ease-out;
  }
  .modal {
    width: 520px;
    max-height: 75vh;
    background: var(--black-1);
    border: 1px solid var(--t-3);
    border-radius: var(--r-lg);
    display: flex;
    flex-direction: column;
    box-shadow: 0 24px 64px rgba(0,0,0,0.6);
  }
  .m-head {
    display: flex; align-items: center; justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-bottom: 1px solid var(--t-3);
  }
  .m-title { font-size: 13px; font-weight: 700; color: var(--t-0); letter-spacing: 0.3px; }
  .m-tabs {
    display: flex;
    border-bottom: 1px solid var(--t-3);
  }
  .m-tab {
    padding: var(--sp-3) var(--sp-4);
    font-size: 12px;
    font-weight: 500;
    color: var(--t-2);
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
  }
  .m-tab:hover { color: var(--t-0); }
  .m-tab.active { color: var(--gold); border-bottom-color: var(--gold); }
  .m-body { flex:1; overflow-y:auto; padding: var(--sp-5); }
  .m-desc { font-size: 12px; color: var(--t-2); margin-bottom: var(--sp-3); line-height: 1.5; }
  .m-prompt {
    width: 100%;
    min-height: 140px;
    background: var(--black-0);
    border: 1px solid var(--t-3);
    border-radius: var(--r);
    color: var(--t-0);
    font-size: 12px;
    padding: var(--sp-3);
    line-height: 1.6;
    resize: vertical;
    font-family: var(--mono);
  }
  .m-prompt:focus { border-color: var(--gold-dim); outline: none; }
  .m-actions { display:flex; align-items:center; gap: var(--sp-2); margin-top: var(--sp-3); }
  .m-btn {
    padding: 6px 16px;
    border-radius: var(--r);
    font-size: 12px;
    font-weight: 500;
    background: var(--black-4);
    color: var(--t-1);
  }
  .m-btn:hover { background: var(--black-5); color: var(--t-0); }
  .m-btn.gold { background: var(--gold); color: var(--black-0); }
  .m-btn.gold:hover { background: var(--gold-dim); }
  .m-btn.danger { background: rgba(239,68,68,0.1); color: var(--red); border: 1px solid rgba(239,68,68,0.15); }
  .m-btn.danger:hover { background: rgba(239,68,68,0.18); }
  .m-ok { font-size: 11px; color: var(--green); }
  .mem-top { display:flex; align-items:center; justify-content:space-between; margin-bottom: var(--sp-4); }
  .mem-top .m-desc { margin-bottom: 0; }
  .mem-empty { text-align:center; padding: var(--sp-8); color: var(--t-2); font-size: 12px; }
  .mem-grid { display:flex; flex-direction:column; gap: 2px; max-height: 280px; overflow-y:auto; }
  .mem-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: var(--sp-2) var(--sp-3);
    border-radius: var(--r);
    background: var(--black-0);
  }
  .mem-row:hover { background: var(--black-3); }
  .mem-info { display:flex; flex-direction:column; gap:1px; flex:1; min-width:0; }
  .mem-name { font-size: 11px; font-family: var(--mono); color: var(--t-0); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .mem-meta { font-size: 10px; color: var(--t-2); }
  .mem-x {
    font-size: 14px; color: var(--t-2); width:22px; height:22px;
    display:flex; align-items:center; justify-content:center;
    border-radius: 3px; opacity: 0.4;
  }
  .mem-x:hover { color: var(--red); opacity: 1; background: var(--black-4); }
</style>
