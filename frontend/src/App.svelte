<script>
  import { marked } from 'marked';
  import { onMount, tick } from 'svelte';
  import {
    SendMessage, SendMessageWithImage, SendMessageWithFile, ToggleIncognito,
    NewChat, ListChats, SwitchChat, DeleteChat,
    GetActiveMessages, GetActiveChatID,
    SelectImage, SelectFile,
    CheckConnection, GetMemoryCount,
    GetSystemPrompt, SetSystemPrompt, ResetSystemPrompt,
    GetIncognitoPrompt, SetIncognitoPrompt,
    ClearAllMemory, ListMemoryFiles, DeleteMemoryFile,
    GetImageBase64,
    StartRecording, StopRecordingAndTranscribe,
    GetRemoteAccessStatus, SetRemoteAccess,
    isWailsEnvironment
  } from './lib/api-bridge.js';

  // EventsOn - only available in Wails
  let _eventsOnReady = null;
  if (isWailsEnvironment()) {
    _eventsOnReady = import('../wailsjs/runtime/runtime.js').then(m => m.EventsOn);
  }

  marked.setOptions({ breaks: true, gfm: true });

  let messages = [];
  let input = '';
  let loading = false;
  let chatEl;
  let chats = [];
  let activeChatId = '';
  let sidebarOpen = window.innerWidth > 768;
  
  // Custom i18n setup
  let lang = localStorage.getItem('memo_lang') || 'tr';
  const dict = {
    tr: {
      newChat: "Yeni Sohbet",
      incognito: "Gizli Mod",
      incognitoActive: "Gizli Mod Aktif",
      memories: "hatıra",
      welcome: "Size nasıl yardımcı olabilirim?",
      inputPlaceholder: "Memo'ya bir mesaj yazın...",
      settings: "Ayarlar",
      general: "Genel",
      systemPrompt: "Sistem İstemi",
      incognitoPrompt: "Gizli Mod İstemi",
      memory: "Hafıza",
      prefs: "Uygulama Tercihleri",
      language: "Dil",
      save: "Kaydet",
      saved: "kaydedildi",
      reset: "Sıfırla",
      clearAll: "Tümünü Temizle",
      remote: "Uzaktan Erişim",
      userProfile: "Kullanıcı",
      freePlan: "Ücretsiz Plan"
    },
    en: {
      newChat: "New Chat",
      incognito: "Incognito",
      incognitoActive: "Incognito Mode Active",
      memories: "memories",
      welcome: "What can I help you with?",
      inputPlaceholder: "Type a message to Memo...",
      settings: "Settings",
      general: "General",
      systemPrompt: "System Prompt",
      incognitoPrompt: "Incognito Prompt",
      memory: "Memory",
      prefs: "Application Preferences",
      language: "Language",
      save: "Save",
      saved: "saved",
      reset: "Reset Default",
      clearAll: "Clear All",
      remote: "Remote Access",
      userProfile: "User Account",
      freePlan: "Free Plan"
    }
  };
  $: t = (key) => dict[lang][key] || key;
  
  function setLanguage(l) {
    lang = l;
    localStorage.setItem('memo_lang', l);
  }
  
  onMount(() => {
    const handleResize = () => { if(window.innerWidth <= 768 && sidebarOpen) sidebarOpen = false; };
    window.addEventListener('resize', handleResize);

    // Fix mobile keyboard pushing content out of view
    let vpHandler;
    if (window.visualViewport) {
      vpHandler = () => {
        const vh = window.visualViewport.height;
        document.documentElement.style.setProperty('--app-height', `${vh}px`);
      };
      vpHandler(); // set initial
      window.visualViewport.addEventListener('resize', vpHandler);
    }

    // Block F12, Ctrl+Shift+I, and right-click context menu (desktop only)
    let blockDevTools, blockContextMenu;
    if (isDesktop) {
      blockDevTools = (e) => {
        if (e.key === 'F12' || (e.ctrlKey && e.shiftKey && e.key === 'I')) {
          e.preventDefault();
        }
      };
      blockContextMenu = (e) => e.preventDefault();
      window.addEventListener('keydown', blockDevTools);
      window.addEventListener('contextmenu', blockContextMenu);
    }

    return () => {
      window.removeEventListener('resize', handleResize);
      if (vpHandler && window.visualViewport) window.visualViewport.removeEventListener('resize', vpHandler);
      if (blockDevTools) window.removeEventListener('keydown', blockDevTools);
      if (blockContextMenu) window.removeEventListener('contextmenu', blockContextMenu);
    };
  });
  let conn = { connected: false, models: [] };
  let memCount = 0;

  let attachedImage = '';
  let attachedFile = '';
  let attachedFileName = '';

  let settingsOpen = false;
  let settingsTab = 'prompt';
  let sysPrompt = '';
  let incognitoPrompt = '';
  let promptSaved = false;
  let memFiles = [];
  let memBusy = false;
  let isIncognito = false;

  // Speech-to-text
  let isRecording = false;
  let isTranscribing = false;

  // Desktop-only features detection
  const isDesktop = isWailsEnvironment();

  // Web file input refs
  let webImageInput;
  let webFileInput;

  // Remote Access
  let remoteEnabled = false;
  let remotePort = 8080;
  let remoteRunning = false;
  let remoteAddresses = [];
  let remoteSaving = false;

  onMount(async () => {
    await refreshAll();
    setInterval(refreshStatus, 30000);

    if (isDesktop && _eventsOnReady) {
      const EventsOn = await _eventsOnReady;
      EventsOn('wails:file-drop', (x, y, paths) => {
        if (paths && paths.length > 0) {
          const file = paths[0];
          const ext = file.split('.').pop().toLowerCase();
          if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp'].includes(ext)) {
            attachedImage = file;
            attachedFile = '';
            attachedFileName = '';
          } else {
            attachedFile = file;
            attachedFileName = file.split(/[/\\]/).pop();
            attachedImage = '';
          }
        }
      });
    }
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
      if (attachedImage) {
        reply = await SendMessageWithImage(text, isDesktop ? attachedImage : webImageFile);
      } else if (attachedFile) {
        reply = await SendMessageWithFile(text, isDesktop ? attachedFile : webFileFile);
      } else {
        reply = await SendMessage(text);
      }
    } catch (e) { reply = '⚠ ' + (e?.message || e); }

    attachedImage = ''; attachedFile = ''; attachedFileName = '';
    webImageFile = null; webFileFile = null;
    messages = [...messages, { id: Date.now()+1, role: 'assistant', text: reply, image:'', file:'', time: ts() }];
    loading = false;
    await tick(); scroll();
    refreshChats();
  }

  function onKey(e) { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send(); } }

  // ─── Attach ───────────────────
  async function pickImage() {
    if (isDesktop) {
      try { const p = await SelectImage(); if (p) { attachedImage=p; attachedFile=''; attachedFileName=''; } } catch(e) {}
    } else {
      webImageInput?.click();
    }
  }
  async function pickFile() {
    if (isDesktop) {
      try { const p = await SelectFile(); if (p) { attachedFile=p; attachedFileName=p.split('/').pop(); attachedImage=''; } } catch(e) {}
    } else {
      webFileInput?.click();
    }
  }
  function clearAttach() { attachedImage=''; attachedFile=''; attachedFileName=''; }

  let webImageFile = null;
  let webFileFile = null;

  // Web file input handlers
  function onWebImage(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    attachedImage = URL.createObjectURL(file);
    attachedFile = '';
    attachedFileName = '';
    webImageFile = file;
    webFileFile = null;
    e.target.value = '';
  }
  function onWebFile(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    attachedFile = URL.createObjectURL(file);
    attachedFileName = file.name;
    attachedImage = '';
    webFileFile = file;
    webImageFile = null;
    e.target.value = '';
  }

  // ─── Sessions ─────────────────
  async function newChat() {
    if (isIncognito) {
      isIncognito = false;
      try { await ToggleIncognito(false); } catch(e) {}
    }
    input = '';
    try { await NewChat(); await refreshChats(); await loadMessages(); } catch(e) {}
  }
  async function switchTo(id) {
    if (isIncognito) {
      isIncognito = false;
      try { await ToggleIncognito(false); } catch(e) {}
    }
    if (id===activeChatId) return; 
    try { await SwitchChat(id); activeChatId=id; await loadMessages(); } catch(e) {}
  }
  async function delChat(e, id) {
    e.stopPropagation();
    try {
      await DeleteChat(id);
      await refreshChats();
      if (activeChatId === id && !isIncognito) await newChat();
    } catch (err) {}
  }

  async function startIncognito() {
    isIncognito = true;
    activeChatId = 'incognito';
    messages = [];
    input = '';
    try { await ToggleIncognito(true); } catch(e) {}
  }

  // ─── Speech to Text ────────────
  async function startMic() {
    if (isRecording || isTranscribing) return;
    try {
      await StartRecording();
      isRecording = true;
    } catch (e) {
      console.error('Mic start error:', e);
      isRecording = false;
    }
  }

  // Tick: stop → put text in input for editing
  async function stopMicToEdit() {
    if (!isRecording) return;
    isRecording = false;
    isTranscribing = true;
    try {
      const text = await StopRecordingAndTranscribe();
      if (text) {
        input = input ? input + ' ' + text : text;
      }
    } catch (e) {
      console.error('STT error:', e);
    }
    isTranscribing = false;
  }

  // Send: stop → transcribe → send immediately
  async function stopMicAndSend() {
    if (!isRecording) return;
    isRecording = false;
    isTranscribing = true;
    try {
      const text = await StopRecordingAndTranscribe();
      if (text) {
        input = text;
        isTranscribing = false;
        await send();
        return;
      }
    } catch (e) {
      console.error('STT error:', e);
    }
    isTranscribing = false;
  }

  // ─── Settings ─────────────────
  async function openSettings() {
    settingsOpen = true; settingsTab = 'prompt'; promptSaved = false;
    try { sysPrompt = await GetSystemPrompt() || ''; } catch(e) { sysPrompt=''; }
    try { incognitoPrompt = await GetIncognitoPrompt() || ''; } catch(e) { incognitoPrompt=''; }
  }
  async function savePrompt() {
    try { await SetSystemPrompt(sysPrompt); promptSaved=true; setTimeout(()=>promptSaved=false,2000); } catch(e) {}
  }
  async function saveIncognitoPrompt() {
    try { await SetIncognitoPrompt(incognitoPrompt); promptSaved=true; setTimeout(()=>promptSaved=false,2000); } catch(e) {}
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

  async function openRemoteTab() {
    settingsTab = 'remote';
    try {
      const status = await GetRemoteAccessStatus();
      remoteEnabled = status.enabled;
      remotePort = status.port;
      remoteRunning = status.running;
      remoteAddresses = status.addresses || [];
    } catch(e) {}
  }

  async function saveRemoteAccess() {
    remoteSaving = true;
    try {
      await SetRemoteAccess(remoteEnabled, remotePort);
      // Refresh status
      const status = await GetRemoteAccessStatus();
      remoteRunning = status.running;
      remoteAddresses = status.addresses || [];
    } catch(e) {
      console.error('Remote access error:', e);
    }
    remoteSaving = false;
  }
</script>

<div class="shell" class:incognito-mode={isIncognito}>
  <input type="file" accept="image/*" style="display:none" bind:this={webImageInput} on:change={onWebImage} />
  <input type="file" style="display:none" bind:this={webFileInput} on:change={onWebFile} />
  <!-- ═══ Sidebar ═══ -->
  {#if sidebarOpen}
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="side-overlay" on:click={() => sidebarOpen = false}></div>
  <aside class="side">
    <div class="side-top">
      <button class="new-session" on:click={newChat}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        <span>{t('newChat')}</span>
      </button>
      <button class="new-session incognito-btn" on:click={startIncognito}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M22 11v1a10 10 0 1 1-9-10"></path><path d="M22 4L12 14.01l-3-3"></path></svg>
        <span>{t('incognito')}</span>
      </button>
    </div>

    <nav class="sessions">
      {#each chats as c}
        <button class="s-item" class:active={c.id === activeChatId} on:click={() => { switchTo(c.id); if (window.innerWidth <= 768) sidebarOpen = false; }}>
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
      <div class="user-profile">
        <div class="user-avatar">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle></svg>
        </div>
        <div class="user-info">
          <span class="user-name">{t('userProfile')}</span>
          <span class="user-plan">{t('freePlan')}</span>
        </div>
        <button class="user-settings-btn" on:click={openSettings}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"></circle><circle cx="12" cy="5" r="1"></circle><circle cx="12" cy="19" r="1"></circle></svg>
        </button>
      </div>
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
      <span class="bar-title">
        {#if isIncognito}
          <div class="incognito-title">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11v1a10 10 0 1 1-9-10"></path><path d="M22 4L12 14.01l-3-3"></path></svg>
            <span>{t('incognitoActive')}</span>
          </div>
        {:else}
          {chats.find(c => c.id === activeChatId)?.title || 'Memo'}
        {/if}
      </span>
      <button class="bar-btn" on:click={openSettings}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
      </button>
    </header>

    <!-- Feed -->
    <div class="feed" bind:this={chatEl}>
      {#if messages.length === 0}
        <div class="welcome">
          <div class="w-mark">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
          </div>
          {#if isIncognito}
            <div class="w-sub" style="color:var(--red);">{t('incognitoActive')}</div>
          {:else}
            <div class="w-sub">{t('welcome')}</div>
          {/if}
        </div>
      {/if}

      {#each messages as m (m.id)}
        <div class="entry" class:memo={m.role==='assistant'} style="animation:fadeIn 120ms ease-out">
          <div class="entry-head">
            <span class="entry-sender">{m.role === 'user' ? 'Buğra' : 'Memo'} ›</span>
            <span class="entry-time">{m.time}</span>
          </div>
          {#if m.image}
            <div class="entry-attach image-attach">
              {#await GetImageBase64(m.image)}
                <span class="loading-img">Loading image...</span>
              {:then src}
                {#if src}
                  <img src={src} alt="attachment" class="chat-img" />
                {:else}
                  <span class="error-img">⚠️ Missing image: {m.image.split(/[/\\]/).pop()}</span>
                {/if}
              {/await}
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
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="1.5"><path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/></svg>
        <span>{attachedImage ? attachedImage.split('/').pop() : attachedFileName}</span>
        <button class="attach-x" on:click={clearAttach}>×</button>
      </div>
    {/if}

    <!-- Input -->
    <div class="input-dock">
      {#if isRecording}
        <!-- Recording mode -->
        <div class="input-row recording-row">
          <button class="rec-action-btn tick-btn" on:click={stopMicToEdit} title="Stop & edit text">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
          </button>
          <div class="waveform">
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
            <div class="wave-bar"></div>
          </div>
          <button class="rec-action-btn send-rec-btn" on:click={stopMicAndSend} title="Stop & send immediately">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
          </button>
        </div>
      {:else if isTranscribing}
        <!-- Transcribing mode -->
        <div class="input-row transcribing-row">
          <span class="spin"></span>
          <span class="transcribing-text">Converting speech...</span>
        </div>
      {:else}
        <!-- Normal mode -->
        <div class="input-row">
          <button class="dock-btn" on:click={pickImage} disabled={loading} title="Attach image">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>
          </button>
          <button class="dock-btn" on:click={pickFile} disabled={loading} title="Attach file">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/></svg>
          </button>
          <button class="dock-btn" on:click={startMic} disabled={loading} title="Voice input">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"/>
              <path d="M19 10v2a7 7 0 0 1-14 0v-2"/>
              <line x1="12" y1="19" x2="12" y2="23"/>
              <line x1="8" y1="23" x2="16" y2="23"/>
            </svg>
          </button>
          <textarea
            bind:value={input}
            on:keydown={onKey}
            placeholder={t('inputPlaceholder')}
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
      {/if}
    </div>
  </main>
</div>

<!-- ═══ Settings Modal ═══ -->
{#if settingsOpen}
<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="overlay" on:click={() => settingsOpen=false}>
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal" on:click|stopPropagation>
    <div class="m-head">
      <span class="m-title">{t('settings')}</span>
      <button class="bar-btn" on:click={() => settingsOpen=false}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
      </button>
    </div>
    <div class="m-tabs">
      <button class="m-tab" class:active={settingsTab==='general'} on:click={() => settingsTab='general'}>{t('general')}</button>
      <button class="m-tab" class:active={settingsTab==='prompt'} on:click={() => settingsTab='prompt'}>{t('systemPrompt')}</button>
      <button class="m-tab" class:active={settingsTab==='incognito'} on:click={() => settingsTab='incognito'}>{t('incognitoPrompt')}</button>
      <button class="m-tab" class:active={settingsTab==='memory'} on:click={openMemTab}>{t('memory')}</button>
      {#if isDesktop}
      <button class="m-tab" class:active={settingsTab==='remote'} on:click={openRemoteTab}>Remote Access</button>
      {/if}
    </div>
    <div class="m-body">
      {#if settingsTab === 'general'}
        <div class="m-section">
          <p class="m-desc">{t('prefs')}</p>
          <div class="setting-row">
            <span class="setting-label">{t('language')}</span>
            <div class="lang-toggle">
              <button class="lang-btn" class:active={lang === 'tr'} on:click={() => setLanguage('tr')}>Türkçe</button>
              <button class="lang-btn" class:active={lang === 'en'} on:click={() => setLanguage('en')}>English</button>
            </div>
          </div>
        </div>
      {:else if settingsTab === 'prompt'}
        <p class="m-desc">Define how Memo behaves. Leave empty for default.</p>
        <textarea class="m-prompt" bind:value={sysPrompt} placeholder="e.g. You are a senior Go developer..." rows="8"></textarea>
        <div class="m-actions">
          <button class="m-btn gold" on:click={savePrompt}>{t('save')}</button>
          <button class="m-btn" on:click={resetPrompt}>{t('reset')}</button>
          {#if promptSaved}<span class="m-ok">✓ {t('saved')}</span>{/if}
        </div>
      {:else if settingsTab === 'incognito'}
        <p class="m-desc" style="color:var(--red);">Define how Memo behaves in Incognito Mode.</p>
        <textarea class="m-prompt" bind:value={incognitoPrompt} placeholder="e.g. You are Memo in Incognito Mode..." style="border-color: rgba(255, 60, 60, 0.3);" rows="8"></textarea>
        <div class="m-actions">
          <button class="m-btn danger" on:click={saveIncognitoPrompt}>{t('save')}</button>
          {#if promptSaved}<span class="m-ok" style="color:var(--red);">✓ {t('saved')}</span>{/if}
        </div>
      {:else if settingsTab === 'memory'}
        <div class="mem-top">
          <p class="m-desc">{memFiles.length} {t('memories')} stored</p>
          <button class="m-btn danger" on:click={clearMem} disabled={memBusy || !memFiles.length}>
            {memBusy ? '...' : t('clearAll')}
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
      {:else if settingsTab === 'remote'}
        <div class="remote-section">
          <p class="m-desc">Enable remote access to use Memo from your phone or other devices on the same network.</p>

          <div class="remote-toggle-row">
            <label class="toggle-label">
              <span>Remote Access</span>
              <button class="toggle-switch" class:on={remoteEnabled} on:click={() => remoteEnabled = !remoteEnabled}>
                <span class="toggle-knob"></span>
              </button>
            </label>
          </div>

          <label class="field remote-field">
            <span>Port</span>
            <input type="number" bind:value={remotePort} min="1024" max="65535" placeholder="8080" />
          </label>

          {#if remoteRunning}
            <div class="remote-status running">
              <span class="remote-dot on"></span>
              <span>Server Running</span>
            </div>
            <div class="remote-addresses">
              <p class="m-desc" style="margin-bottom:8px;">Connect from your phone:</p>
              {#each remoteAddresses as addr}
                <div class="remote-addr">{addr}</div>
              {/each}
            </div>
          {:else if remoteEnabled}
            <div class="remote-status">
              <span class="remote-dot"></span>
              <span>Server Stopped</span>
            </div>
          {/if}

          <div class="m-actions" style="margin-top:16px;">
            <button class="m-btn gold" on:click={saveRemoteAccess} disabled={remoteSaving}>
              {remoteSaving ? '...' : 'Save & Apply'}
            </button>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>
{/if}

<style>
  /* ═══════════════════════════════════
     SHELL LAYOUT
  ═══════════════════════════════════ */
  .shell { height: var(--app-height, 100dvh); display:flex; overflow:hidden; }

  /* ─── SIDEBAR ─── */
  .side {
    width: 240px;
    background: var(--bg-panel);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    border-right: 1px solid var(--text-dim);
  }
  .side-top { padding: var(--sp-4); }
  .new-session {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    width: 100%;
    padding: var(--sp-3) var(--sp-4);
    color: var(--accent);
    font-size: 14px;
    font-weight: 500;
    letter-spacing: 0.3px;
    border-radius: var(--r-md);
  }
  .new-session:hover { background: var(--accent-muted); }
  
  .incognito-btn { color: var(--red); margin-top: var(--sp-2); }
  .incognito-btn:hover { background: rgba(255, 60, 60, 0.1); }
  .incognito-mode .bar-title { color: var(--red); font-weight: 500; }

  .sessions { flex:1; overflow-y:auto; padding: 0 var(--sp-2); }
  .s-item {
    display: flex;
    align-items: center;
    width: 100%;
    padding: var(--sp-3) var(--sp-3);
    border-bottom: 1px solid var(--text-dim);
    text-align: left;
    color: var(--text-main);
    font-size: 14px;
    gap: var(--sp-2);
  }
  .s-item:last-child { border-bottom: none; }
  .s-item:hover { color: var(--text-main); background: var(--bg-element); }
  .s-item.active { color: var(--accent); background: var(--accent-muted); }
  .s-info { flex:1; min-width:0; display:flex; flex-direction:column; gap:1px; }
  .s-title {
    font-size: 14px;
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .s-time { font-size: 11px; color: var(--text-muted); }
  .s-del {
    opacity: 0;
    width: 24px; height: 24px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px;
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .s-item:hover .s-del { opacity: 1; }
  .s-del:hover { background: var(--border-soft); color: var(--red); }

  .side-bottom {
    padding: var(--sp-3) var(--sp-3);
    border-top: 1px solid var(--border-soft);
    background: var(--bg-panel);
    flex-shrink: 0;
  }
  .user-profile {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-2);
    border-radius: var(--r-md);
    transition: background 0.15s;
    cursor: pointer;
  }
  .user-profile:hover {
    background: var(--bg-hover);
  }
  .user-avatar {
    width: 32px; height: 32px;
    border-radius: 50%;
    background: var(--bg-element);
    color: var(--text-muted);
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0;
  }
  .user-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .user-name {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .user-plan {
    font-size: 11px;
    color: var(--text-muted);
  }
  .user-settings-btn {
    width: 28px; height: 28px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 50%;
    color: var(--text-muted);
    opacity: 0.6;
    flex-shrink: 0;
  }
  .user-profile:hover .user-settings-btn { opacity: 1; }
  .user-settings-btn:hover { background: var(--border-soft); color: var(--text-main); }
  .incognito-title {
    display: flex; align-items: center; gap: 6px;
  }

  /* ─── MAIN AREA ─── */
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-app);
  }

  .bar {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-2) var(--sp-4);
    border-bottom: 1px solid var(--text-dim);
    background: var(--bg-panel);
    flex-shrink: 0;
    height: 44px;
  }
  .bar-title {
    flex: 1;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-main);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    letter-spacing: 0.2px;
  }
  .bar-btn {
    width: 36px; height: 36px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--r-md);
    color: var(--text-muted);
  }
  .bar-btn:hover { background: var(--border-soft); color: var(--text-main); }

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
    color: var(--accent);
    letter-spacing: -1px;
    opacity: 0.7;
  }
  .w-sub {
    font-size: 11px;
    color: var(--text-muted);
    letter-spacing: 1px;
    text-transform: uppercase;
  }

  .entry {
    max-width: 800px;
    margin: 0 auto var(--sp-6);
    padding: 0 var(--sp-4);
    width: 100%;
  }
  .entry-head {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    margin-bottom: var(--sp-2);
  }
  .entry-sender {
    font-weight: 600;
    font-size: 15px;
    color: var(--text-main);
  }
  .entry-time {
    font-size: 12px;
    color: var(--text-muted);
  }
  .entry-attach {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: 13px;
    color: var(--text-muted);
    margin-bottom: var(--sp-1);
  }
  .image-attach {
    display: block;
    margin-top: var(--sp-2);
    margin-bottom: var(--sp-2);
  }
  .chat-img {
    max-width: 100%;
    max-height: 350px;
    border-radius: var(--r-md);
    display: block;
  }
  .loading-img, .error-img {
    font-size: 12px;
    color: var(--text-dim);
    font-style: italic;
  }
  .entry-body {
    font-size: 15px;
    line-height: 1.7;
    color: var(--text-main);
  }
  .thinking { color: var(--text-muted); }
  .cursor-blink { animation: pulse 1s infinite; color: var(--accent); }

  /* ─── ATTACHMENT ROW ─── */
  .attach-row {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    padding: var(--sp-1) var(--sp-6);
    font-size: 11px;
    color: var(--accent);
  }
  .attach-x {
    font-size: 14px;
    color: var(--text-muted);
    width: 18px; height: 18px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 3px;
  }
  .attach-x:hover { color: var(--red); }

  /* ─── INPUT DOCK ─── */
  .input-dock {
    padding: var(--sp-4) var(--sp-5) var(--sp-6); /* add padding bottom for floating look */
    background: transparent;
    border-top: none;
    display: flex;
    justify-content: center;
  }
  .input-row {
    display: flex;
    align-items: flex-end;
    gap: var(--sp-1);
    width: 100%;
    max-width: 800px;
    margin: 0 auto;
    background: var(--bg-app);
    border: 1px solid var(--border-soft);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-md);
    padding: var(--sp-2) var(--sp-2);
    transition: box-shadow 0.2s ease;
  }
  .input-row:focus-within {
    box-shadow: var(--shadow-lg);
    border-color: var(--border-hover);
  }
  .dock-btn {
    width: 40px; height: 40px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 50%;
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .dock-btn:hover:not(:disabled) { color: var(--accent); background: var(--bg-hover); }
  .input-row textarea {
    flex: 1;
    background: transparent;
    border: none;
    color: var(--text-main);
    font-size: 15px;
    line-height: 1.5;
    resize: none;
    min-height: 24px;
    max-height: 200px;
    padding: 8px var(--sp-2);
    outline: none;
    font-family: var(--sans);
  }
  .input-row textarea::placeholder { color: var(--text-dim); }
  .input-row textarea:disabled { opacity: 0.35; }
  .send-btn {
    width: 38px; height: 38px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--r-md);
    color: var(--accent);
    flex-shrink: 0;
  }
  .send-btn:hover:not(:disabled) { background: var(--accent-muted); }
  .send-btn:disabled { color: var(--text-dim); }
  .spin {
    width: 14px; height: 14px;
    border: 1.5px solid var(--text-dim);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.5s linear infinite;
  }

  /* ─── RECORDING MODE ─── */
  .recording-row {
    justify-content: center;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-2) 0;
  }
  .waveform {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 3px;
    height: 38px;
    max-width: 400px;
  }
  .wave-bar {
    width: 3px;
    border-radius: 2px;
    background: var(--red);
    animation: waveAnim 1.2s ease-in-out infinite;
  }
  .wave-bar:nth-child(1)  { animation-delay: 0.0s; }
  .wave-bar:nth-child(2)  { animation-delay: 0.1s; }
  .wave-bar:nth-child(3)  { animation-delay: 0.2s; }
  .wave-bar:nth-child(4)  { animation-delay: 0.3s; }
  .wave-bar:nth-child(5)  { animation-delay: 0.4s; }
  .wave-bar:nth-child(6)  { animation-delay: 0.5s; }
  .wave-bar:nth-child(7)  { animation-delay: 0.6s; }
  .wave-bar:nth-child(8)  { animation-delay: 0.7s; }
  .wave-bar:nth-child(9)  { animation-delay: 0.6s; }
  .wave-bar:nth-child(10) { animation-delay: 0.5s; }
  .wave-bar:nth-child(11) { animation-delay: 0.4s; }
  .wave-bar:nth-child(12) { animation-delay: 0.3s; }
  .wave-bar:nth-child(13) { animation-delay: 0.2s; }
  .wave-bar:nth-child(14) { animation-delay: 0.1s; }
  .wave-bar:nth-child(15) { animation-delay: 0.0s; }
  .wave-bar:nth-child(16) { animation-delay: 0.1s; }

  @keyframes waveAnim {
    0%, 100% { height: 4px; opacity: 0.4; }
    50%      { height: 28px; opacity: 1; }
  }

  .rec-action-btn {
    width: 42px;
    height: 42px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background 0.15s, transform 0.1s;
  }
  .rec-action-btn:active { transform: scale(0.92); }

  .tick-btn {
    background: rgba(34, 197, 94, 0.12);
    color: var(--green);
    border: 1px solid rgba(34, 197, 94, 0.25);
  }
  .tick-btn:hover {
    background: rgba(34, 197, 94, 0.22);
  }

  .send-rec-btn {
    background: var(--accent-muted);
    color: var(--accent);
    border: 1px solid rgba(212, 175, 55, 0.25);
  }
  .send-rec-btn:hover {
    background: rgba(212, 175, 55, 0.22);
  }

  .transcribing-row {
    justify-content: center;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-3) 0;
  }
  .transcribing-text {
    font-size: 13px;
    color: var(--text-muted);
    letter-spacing: 0.3px;
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
    background: var(--bg-panel);
    border: 1px solid var(--text-dim);
    border-radius: var(--r-lg);
    display: flex;
    flex-direction: column;
    box-shadow: 0 24px 64px rgba(0,0,0,0.6);
  }
  .m-head {
    display: flex; align-items: center; justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-bottom: 1px solid var(--text-dim);
  }
  .m-title { font-size: 13px; font-weight: 700; color: var(--text-main); letter-spacing: 0.3px; }
  .m-tabs {
    display: flex;
    border-bottom: 1px solid var(--text-dim);
  }
  .m-tab {
    padding: var(--sp-3) var(--sp-4);
    font-size: 12px;
    font-weight: 500;
    color: var(--text-muted);
    border-bottom: 2px solid transparent;
    margin-bottom: -1px;
  }
  .m-tab:hover { color: var(--text-main); }
  .m-tab.active { color: var(--accent); border-bottom-color: var(--accent); }
  .m-body { flex:1; overflow-y:auto; padding: var(--sp-5); }
  .m-desc { font-size: 12px; color: var(--text-muted); margin-bottom: var(--sp-3); line-height: 1.5; }
  .m-section { display: flex; flex-direction: column; gap: var(--sp-3); }
  .setting-row { display: flex; align-items: center; justify-content: space-between; padding: var(--sp-3) 0; border-bottom: 1px solid var(--border-soft); }
  .setting-row:last-child { border-bottom: none; }
  .setting-label { font-size: 14px; color: var(--text-main); font-weight: 500; }
  .lang-toggle { display: flex; background: var(--bg-element); border-radius: var(--r-md); padding: 4px; gap: 2px; }
  .lang-btn { padding: 4px 12px; border-radius: 6px; font-size: 12px; color: var(--text-muted); font-weight: 600; }
  .lang-btn.active { background: var(--bg-app); color: var(--text-main); box-shadow: var(--shadow-sm); }
  .m-prompt {
    width: 100%;
    min-height: 140px;
    background: var(--bg-app);
    border: 1px solid var(--text-dim);
    border-radius: var(--r-md);
    color: var(--text-main);
    font-size: 12px;
    padding: var(--sp-3);
    line-height: 1.6;
    resize: vertical;
    font-family: var(--mono);
  }
  .m-prompt:focus { border-color: var(--accent-hover); outline: none; }
  .m-actions { display:flex; align-items:center; gap: var(--sp-2); margin-top: var(--sp-3); }
  .m-btn {
    padding: 6px 16px;
    border-radius: var(--r-md);
    font-size: 12px;
    font-weight: 500;
    background: var(--border-soft);
    color: var(--text-main);
  }
  .m-btn:hover { background: var(--border-hover); color: var(--text-main); }
  .m-btn.gold { background: var(--accent); color: var(--bg-app); }
  .m-btn.gold:hover { background: var(--accent-hover); }
  .m-btn.danger { background: rgba(239,68,68,0.1); color: var(--red); border: 1px solid rgba(239,68,68,0.15); }
  .m-btn.danger:hover { background: rgba(239,68,68,0.18); }
  .m-ok { font-size: 11px; color: var(--green); }
  .mem-top { display:flex; align-items:center; justify-content:space-between; margin-bottom: var(--sp-4); }
  .mem-top .m-desc { margin-bottom: 0; }
  .mem-empty { text-align:center; padding: var(--sp-8); color: var(--text-muted); font-size: 12px; }
  .mem-grid { display:flex; flex-direction:column; gap: 2px; max-height: 280px; overflow-y:auto; }
  .mem-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: var(--sp-2) var(--sp-3);
    border-radius: var(--r-md);
    background: var(--bg-app);
  }
  .mem-row:hover { background: var(--bg-hover); }
  .mem-info { display:flex; flex-direction:column; gap:1px; flex:1; min-width:0; }
  .mem-name { font-size: 11px; font-family: var(--mono); color: var(--text-main); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .mem-meta { font-size: 10px; color: var(--text-muted); }
  .mem-x {
    font-size: 14px; color: var(--text-muted); width:22px; height:22px;
    display:flex; align-items:center; justify-content:center;
    border-radius: 3px; opacity: 0.4;
  }
  .mem-x:hover { color: var(--red); opacity: 1; background: var(--border-soft); }

  /* ═══ REMOTE ACCESS TAB ═══ */
  .remote-section { display: flex; flex-direction: column; gap: var(--sp-3); }
  .remote-toggle-row {
    display: flex; align-items: center; justify-content: space-between;
    padding: var(--sp-3) 0;
  }
  .toggle-label {
    display: flex; align-items: center; justify-content: space-between;
    width: 100%; font-size: 13px; color: var(--text-main); cursor: pointer;
  }
  .toggle-switch {
    position: relative; width: 44px; height: 24px; border-radius: 12px;
    background: var(--border-soft); border: 1px solid var(--text-dim);
    transition: background 200ms, border-color 200ms; cursor: pointer;
  }
  .toggle-switch.on { background: var(--accent); border-color: var(--accent-hover); }
  .toggle-knob {
    position: absolute; top: 2px; left: 2px;
    width: 18px; height: 18px; border-radius: 50%;
    background: var(--text-main); transition: transform 200ms;
  }
  .toggle-switch.on .toggle-knob { transform: translateX(20px); }
  .remote-field { margin-top: var(--sp-1); }
  .remote-field span { font-size: 12px; color: var(--text-muted); }
  .remote-field input {
    width: 100%; font-size: 13px; padding: 8px 12px;
    background: var(--bg-app); border: 1px solid var(--text-dim);
    border-radius: var(--r-md); color: var(--text-main);
  }
  .remote-field input:focus { border-color: var(--accent-hover); outline: none; }
  .remote-status {
    display: flex; align-items: center; gap: var(--sp-2);
    font-size: 12px; color: var(--text-muted); padding: var(--sp-2) 0;
  }
  .remote-status.running { color: var(--green); }
  .remote-dot {
    width: 7px; height: 7px; border-radius: 50%;
    background: var(--red);
  }
  .remote-dot.on {
    background: var(--green);
    box-shadow: 0 0 8px rgba(74, 222, 128, 0.5);
  }
  .remote-addresses { margin-top: var(--sp-2); }
  .remote-addr {
    font-family: var(--mono); font-size: 13px;
    color: var(--accent); padding: var(--sp-2) var(--sp-3);
    background: var(--bg-app); border: 1px solid var(--border-soft);
    border-radius: var(--r-md); margin-bottom: var(--sp-1);
    word-break: break-all;
  }

  /* ═══════════════════════════════════
     RESPONSIVE (MOBILE)
  ═══════════════════════════════════ */
  @media (max-width: 768px) {
    .side {
      position: fixed;
      z-index: 50;
      height: 100%;
      width: 280px;
      left: 0;
      top: 0;
      box-shadow: 4px 0 24px rgba(0,0,0,0.5);
    }
    .side-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0,0,0,0.6);
      z-index: 40;
      backdrop-filter: blur(2px);
    }
    .main { width: 100%; }
    .bar { padding: var(--sp-2) var(--sp-3); height: 48px; }
    .bar-title { font-size: 13px; }
    .bar-btn { width: 40px; height: 40px; min-width: 40px; }
    .feed {
      padding: var(--sp-3) var(--sp-2);
    }
    .entry { max-width: 100%; padding: var(--sp-2) var(--sp-3); }
    .entry-body { font-size: 14px; line-height: 1.6; }
    .entry-sender { font-size: 13px; }
    .w-mark { font-size: 28px; }
    .w-sub { font-size: 10px; }
    .input-dock {
      padding: var(--sp-3) var(--sp-2);
      padding-bottom: max(var(--sp-3), env(safe-area-inset-bottom));
      background: var(--bg-panel);
      border-top: 1px solid var(--text-dim);
    }
    .input-row {
      max-width: 100%;
      gap: 2px;
    }
    .input-row textarea { font-size: 16px; padding: var(--sp-2) var(--sp-1); }
    .dock-btn { width: 40px; height: 40px; min-width: 40px; }
    .send-btn { width: 42px; height: 42px; min-width: 42px; }
    .attach-row { padding: var(--sp-1) var(--sp-3); }
    .modal { width: 94%; max-height: 90vh; margin: var(--sp-3); }
    .m-head { padding: var(--sp-3) var(--sp-4); }
    .m-tabs { overflow-x: auto; -webkit-overflow-scrolling: touch; }
    .m-tab { white-space: nowrap; padding: var(--sp-3) var(--sp-3); font-size: 11px; }
    .m-body { padding: var(--sp-4); }
    .m-prompt { font-size: 14px; min-height: 120px; }
    .recording-row { gap: var(--sp-2); }
    .rec-action-btn { width: 46px; height: 46px; }
    .waveform { max-width: 250px; }
    .chat-img { max-height: 250px; }
    .remote-addr { font-size: 12px; }
  }

  /* Extra small screens */
  @media (max-width: 400px) {
    .side { width: 260px; }
    .bar-title { font-size: 12px; }
    .entry-body { font-size: 13px; }
    .dock-btn { width: 36px; height: 36px; min-width: 36px; }
  }
</style>
