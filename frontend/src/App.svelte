<script>
  import { marked } from 'marked';
  import { onMount, tick } from 'svelte';
  // @ts-ignore
  const { EventsOn, EventsOff } = (typeof window !== 'undefined' && window.runtime) || {};
  import {
    SendMessage, SendMessageWithImage, SendMessageWithFile,
    SendMessageStream, SendMessageWithImageStream, SendMessageWithFileStream,
    ToggleIncognito,
    NewChat, ListChats, SwitchChat, DeleteChat,
    GetActiveMessages, GetActiveChatID,
    SelectImage, SelectFile,
    CheckConnection, GetMemoryCount,
    GetSystemPrompt, SetSystemPrompt, ResetSystemPrompt,
    GetIncognitoPrompt, SetIncognitoPrompt,
    ClearAllMemory, ListMemoryFiles, DeleteMemoryFile,
    GetImageBase64,
    StartRecording, StopRecordingAndTranscribe,
    GetVersion,
    isWailsEnvironment,
    ListLocalModels, GetLocalModelStatus, StartLocalModel, StopLocalModel, StartEmbeddingModel, StopEmbeddingModel, GetEmbeddingModelStatus, DetectGPU, GetDownloadProgress, DownloadModel, CancelDownload, SearchModels, GetModelFiles, DeleteLocalModel, SetRemoteAccess, GetRemoteAccessStatus, CheckLlamaInstallation, InstallLlamaServer
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
      freePlan: "Ücretsiz Plan",
      about: "Hakkında",
      wizardTitle: "Memo'ya Hoş Geldiniz",
      wizardDesc: "Kişisel AI asistanınızı birlikte kuralım.",
      nameLabel: "Adınız Soyadınız",
      systemPromptLabel: "Özel Sistem İstemi (Opsiyonel)",
      systemPromptDesc: "Boş bırakırsanız varsayılan kullanılacaktır.",
      checkLM: "LM-Studio Bağlantısı",
      checkModels: "Yüklü Modeller",
      lmStudioWarning: "Lütfen LM-Studio'yu açın, bir model yükleyin ve Local Server'ı başlatın (Port: 1234).",
      refresh: "Yenile",
      ready: "Başla Devam Et",
      resetSetup: "Kurulumu Sıfırla",
      aboutDev: "Geliştirici",
      aboutVisionTitle: "Vizyon ve Misyon",
      aboutVisionText: "Memo, tamamen yerel bilgisayarınızda çalışan, sizin konuşmalarınızı ve tercihlerinizi zamanla öğrenip kalıcı hafızasına kazıyan özel bir yapay zeka asistanıdır. Asıl amaç, bulut teknolojilere muhtaç kalmadan, özgürce ve güvenle kendi bilgisayarında barındırabileceğiniz akıllı bir asistan yaratmaktır.",
      aboutPrivacyTitle: "Şeffaflık Raporu & Gizlilik İlkeleri",
      aboutPrivacyText: "Tüm mesaj geçmişiniz, vektör (RAG) hafızası, dosyalarınız ve ses kayıtlarınız cihazınızda (lokal ortamda) kapalı bir devre olarak yaşar. Dış internete veya 3. parti API veritabanlarına kesinlikle log veya bilgi gönderilmez. Uygulamanız ve zihniniz %100 size aittir ve her zaman güvenliğiniz ön plandadır.",
      addModel: "Model Ekle",
      modelDesc: "Hugging Face'den modelinizi bulun ve Repo ID'sini yapıştırın.",
      repoIdPlaceholder: "Repo ID (örn: google/gemma-3-4b-it)",
      getFiles: "Dosyaları Getir",
      vramInsufficient: "⚠️ VRAM Yetersiz",
      gpuCompatible: "✓ GPU Uyumlu",
      downloadedModels: "İndirilen Modeller",
      noModels: "Henüz model indirilmedi. Aramaya başlayın.",
      download: "İndir",
      thinking: "Memo düşünüyor...",
      running: "Çalışıyor",
      embedding: "Hafıza Motoru",
      modelAction: "İşlemler",
      stop: "Durdur",
      start: "Başlat",
      useAsMemory: "Hafıza Olarak Kullan",
      delete: "Sil",
      speed: "Hız",
      totalTokens: "Toplam Token",
      duration: "Süre",
      finishReason: "Bitiş Nedeni",
      loading: "Yükleniyor..."
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
      freePlan: "Free Plan",
      about: "About",
      wizardTitle: "Welcome to Memo",
      wizardDesc: "Let's set up your personal AI assistant.",
      nameLabel: "Your Name and Surname",
      systemPromptLabel: "Custom System Prompt (Optional)",
      systemPromptDesc: "Leave blank to use the default prompt.",
      checkLM: "LM-Studio Connection",
      checkModels: "Loaded Models",
      lmStudioWarning: "Please open LM-Studio, load a model, and start the Local Server (Port: 1234).",
      refresh: "Refresh",
      ready: "Ready to Go",
      resetSetup: "Reset Setup",
      aboutDev: "Developer",
      aboutVisionTitle: "Vision and Mission",
      aboutVisionText: "Memo is a specialized AI assistant that runs entirely locally on your computer, learning your conversations and preferences over time and etching them into its persistent memory. The main goal is to create a smart assistant that you can freely and securely host on your own machine without relying on cloud technologies.",
      aboutPrivacyTitle: "Transparency & Privacy Principles",
      aboutPrivacyText: "All your message history, vector (RAG) memory, files, and voice recordings live in a closed circuit locally on your device. Absolutely no logs or information are sent to the external internet or 3rd party API databases. Your app and your mind are 100% yours, with security always at the forefront.",
      addModel: "Add Model",
      modelDesc: "Find your model on Hugging Face and paste the Repo ID.",
      repoIdPlaceholder: "Repo ID (e.g., google/gemma-3-4b-it)",
      getFiles: "Get Files",
      vramInsufficient: "⚠️ Insufficient VRAM",
      gpuCompatible: "✓ GPU Compatible",
      downloadedModels: "Downloaded Models",
      noModels: "No models downloaded yet. Search above to get started.",
      download: "Download",
      thinking: "Memo is thinking...",
      running: "Running",
      embedding: "Memory Engine",
      modelAction: "Actions",
      stop: "Stop",
      start: "Start",
      useAsMemory: "Use as Memory",
      delete: "Delete",
      speed: "Speed",
      totalTokens: "Total Tokens",
      duration: "Duration",
      finishReason: "Finish Reason",
      loading: "Loading..."
    }
  };
  $: t = (key) => dict[lang][key] || key;
  
  function setLanguage(l) {
    lang = l;
    localStorage.setItem('memo_lang', l);
  }
  
  // Wizard Setup Variables
  let showSetup = localStorage.getItem('memo_setup_complete') !== 'true';
  let setupName = '';
  let setupSurname = '';
  let setupPrompt = '';
  let setupLMStatus = false;
  let setupModelStatus = false;
  let setupChecking = false;
  let appVersion = '...';
  
  async function checkSetupConnection() {
    setupChecking = true;
    try {
      const s = await CheckConnection();
      setupLMStatus = s && s.connected;
      setupModelStatus = s && s.models && s.models.length > 0;
    } catch(e) {
      setupLMStatus = false;
      setupModelStatus = false;
    }
    setupChecking = false;
  }
  
  async function finishSetup() {
    let fullName = (setupName.trim() + " " + setupSurname.trim()).trim();
    if (!fullName) fullName = "User";
    
    let finalPrompt = setupPrompt.trim();
    if (!finalPrompt) {
      finalPrompt = `You are Memo, a highly capable AI assistant. You are speaking with ${fullName}.

Core Directives:
- You have persistent memory. You remember past conversations and use that context naturally.
- You are model-agnostic — regardless of the underlying LLM, you maintain your identity as Memo.
- Be helpful, accurate, and thoughtful in every response.
- When you recall something from a past conversation, integrate it naturally without saying "I recall" or "As we discussed".
- Adapt to the user's language. If they write in Turkish, respond in Turkish. If English, respond in English.`;
    }
    await SetSystemPrompt(finalPrompt);
    localStorage.setItem('memo_setup_complete', 'true');
    showSetup = false;
  }
  
  onMount(() => {
    if (showSetup) {
      checkSetupConnection();
    }
    
    GetVersion().then(v => appVersion = v);
    
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
  let settingsTab = 'general';
  
  let currentView = 'chat'; // 'chat' or 'models'
  let isLlamaInstalled = false;
  let isInstalling = false;
  let installFailed = false;
  let installLog = [];
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

  // Model Store
  let modelSearchQuery = '';
  let modelSearchResults = [];
  let modelSearchError = '';
  let modelSearching = false;
  let expandedModel = null;
  let modelFiles = [];
  let modelFilesLoading = false;
  let localModels = [];
  let localModelLoading = false;
  let modelStarting = '';
  let modelStopping = false;
  let localModelStatus = { running: false, model_name: '', gpu: { type: 'cpu', name: 'N/A' } };
  let downloadProgress = { active: false, percent: 0, speed: '', downloaded: 0, total_bytes: 0 };
  let downloadPollTimer = null;
  let gpuInfo = { type: 'cpu', name: 'N/A', vram_mb: 0 };

  onMount(async () => {
    await refreshAll();
    if (window.runtime && window.runtime.EventsOn) {
      window.runtime.EventsOn("llama:install-log", (msg) => {
        installLog = [...installLog, msg];
        setTimeout(() => {
          const el = document.getElementById("install-log-box");
          if (el) el.scrollTop = el.scrollHeight;
        }, 50);
      });
    }

    setInterval(refreshStatus, 30000);

    if (isDesktop && _eventsOnReady) {
      const EventsOn = await _eventsOnReady;
      EventsOn('memory:error', (msg) => {
        console.warn('[Memory Error]', msg);
      });
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
  onMount(() => {
    EventsOn('chat:chunk', (chunk) => {
      if (chunk.content) {
        if (messages.length > 0) {
          const last = messages[messages.length - 1];
          if (last.role === 'assistant' && last.isStreaming) {
            last.text += chunk.content;
            messages = [...messages];
            scroll();
          }
        }
      }
    });

    EventsOn('chat:done', (chunk) => {
      if (messages.length > 0) {
        const last = messages[messages.length - 1];
        if (last.role === 'assistant' && last.isStreaming) {
          last.isStreaming = false;
          if (chunk.stats) {
            last.stats = chunk.stats;
          }
          messages = [...messages];
          loading = false;
          refreshChats();
        }
      }
    });

    EventsOn('chat:error', (err) => {
      if (messages.length > 0) {
        const last = messages[messages.length - 1];
        if (last.role === 'assistant' && last.isStreaming) {
          last.text += '\n\n' + err;
          last.isStreaming = false;
        }
      }
      loading = false;
    });

    return () => {
      EventsOff('chat:chunk');
      EventsOff('chat:done');
      EventsOff('chat:error');
    };
  });

  async function send() {
    const msg = input.trim();
    if ((!msg && !attachedImage && !attachedFile) || loading) return;
    const text = msg || '(file attached)';
    input = '';
    loading = true;
    messages = [...messages, { id: Date.now(), role: 'user', text, image: attachedImage, file: attachedFileName, time: ts() }];
    await tick(); scroll();

    // Placeholder for assistant response
    messages = [...messages, { 
      id: Date.now()+1, 
      role: 'assistant', 
      text: '', 
      image:'', 
      file:'', 
      time: ts(),
      isStreaming: true 
    }];
    messages = [...messages]; // trigger update

    try {
      if (attachedImage) {
        SendMessageWithImageStream(text, isDesktop ? attachedImage : webImageFile);
      } else if (attachedFile) {
        SendMessageWithFileStream(text, isDesktop ? attachedFile : webFileFile);
      } else {
        SendMessageStream(text);
      }
    } catch (e) { 
      const last = messages[messages.length - 1];
      last.text = '⚠ ' + (e?.message || e);
      last.isStreaming = false;
      loading = false;
      messages = [...messages];
    }

    attachedImage = ''; attachedFile = ''; attachedFileName = '';
    webImageFile = null; webFileFile = null;
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
  // ─── Model Store ───────────────
  async function checkLlamaStatus() {
    isLlamaInstalled = await CheckLlamaInstallation();
  }

  async function openModelsTab() {
    currentView = 'models';
    settingsOpen = false;
    await checkLlamaStatus();
    
    if (isLlamaInstalled) {
      localModelLoading = true;
      try {
        [localModels, localModelStatus, gpuInfo, embeddingModelStatus] = await Promise.all([
          ListLocalModels() || [],
          GetLocalModelStatus(),
          DetectGPU(),
          GetEmbeddingModelStatus()
        ]);
        localModels = localModels || [];
      } catch(e) { console.error('Models tab error:', e); }
      localModelLoading = false;
      pollDownloadProgress();
    }
  }

  async function startInstallation() {
    isInstalling = true;
    installFailed = false;
    installLog = [];
    try {
      await InstallLlamaServer();
      await checkLlamaStatus();
      if (isLlamaInstalled) {
         openModelsTab();
      } else {
         installFailed = true;
      }
    } catch(e) {
      installLog = [...installLog, "\\nERROR: " + e];
      installFailed = true;
    }
    isInstalling = false;
  }

  let manualRepoID = '';
  async function fetchRepoFiles() {
    if (!manualRepoID.trim()) return;
    modelSearching = true;
    modelSearchError = '';
    modelFiles = [];
    try {
      modelFiles = await GetModelFiles(manualRepoID.trim()) || [];
      if (modelFiles.length === 0) {
        modelSearchError = 'Bu repo altında .gguf dosyası bulunamadı veya repo geçersiz.';
      }
    } catch(e) {
      modelSearchError = 'Hata: ' + (e?.message || e);
    }
    modelSearching = false;
  }

  function onSearchKey(e) { if (e.key === 'Enter') fetchRepoFiles(); }

  async function expandModel(repoID) {
    if (expandedModel === repoID) { expandedModel = null; modelFiles = []; return; }
    expandedModel = repoID;
    modelFilesLoading = true;
    try {
      modelFiles = await GetModelFiles(repoID) || [];
    } catch(e) { modelFiles = []; }
    modelFilesLoading = false;
  }

  async function startDownload(repoID, filename) {
    try {
      await DownloadModel(repoID, filename);
      pollDownloadProgress();
    } catch(e) { console.error('Download error:', e); }
  }

  function pollDownloadProgress() {
    if (downloadPollTimer) clearInterval(downloadPollTimer);
    downloadPollTimer = setInterval(async () => {
      try {
        downloadProgress = await GetDownloadProgress();
        if (!downloadProgress.active) {
          clearInterval(downloadPollTimer);
          downloadPollTimer = null;
          // Refresh local models list
          localModels = await ListLocalModels() || [];
        }
      } catch(e) {
        clearInterval(downloadPollTimer);
        downloadPollTimer = null;
      }
    }, 500);
  }

  async function cancelDL() {
    try { await CancelDownload(); } catch(e) {}
  }

  async function deleteModel(path) {
    try {
      await DeleteLocalModel(path);
      localModels = localModels.filter(m => m.path !== path);
    } catch(e) { console.error('Delete error:', e); }
  }

  // Model Config State
  let configModalOpen = false;
  let modelToStart = null;
  let configCtxSize = 4096;
  let configGpuLayers = -1;
  let configPort = 8081;
  let embedModelEnabled = false;
  let selectedEmbedModel = null;
  let embeddingModelStatus = { running: false };

  function getAvailableEmbedModels() {
    if (!localModels) return [];
    return localModels.filter(m => m.is_embedding);
  }

  function openModelConfig(model) {
    modelToStart = model;
    configCtxSize = 4096;
    configPort = 8081;
    if (gpuInfo && gpuInfo.vram_mb > 0) {
      configGpuLayers = -1;
    } else {
      configGpuLayers = 0;
    }
    // Auto-detect embedding models and pre-select if available
    const embedModels = getAvailableEmbedModels();
    if (embedModels.length > 0) {
      embedModelEnabled = true;
      selectedEmbedModel = embedModels[0];
    } else {
      embedModelEnabled = false;
      selectedEmbedModel = null;
    }
    configModalOpen = true;
  }

  async function confirmStartModel() {
    if (!modelToStart) return;
    configModalOpen = false;
    modelStarting = modelToStart.path;
    try {
      await StartLocalModel(modelToStart.path, parseInt(configCtxSize), parseInt(configPort), parseInt(configGpuLayers));
      localModelStatus = await GetLocalModelStatus();

      // Start embedding model alongside if user opted in
      if (embedModelEnabled && selectedEmbedModel) {
        try {
          await StartEmbeddingModel(selectedEmbedModel.path, 0); // CPU for embedding (small model)
          embeddingModelStatus = await GetEmbeddingModelStatus();
        } catch(e) {
          console.error('Embedding model start error:', e);
        }
      }
    } catch(e) {
      console.error('Start model error:', e);
      alert('Failed to start model: ' + (e?.message || e));
    }
    modelStarting = '';
    modelToStart = null;
  }

  async function stopModel() {
    modelStopping = true;
    try {
      // Stop embedding model first if running
      if (embeddingModelStatus.running) {
        await StopEmbeddingModel();
        embeddingModelStatus = { running: false };
      }
      await StopLocalModel();
      localModelStatus = await GetLocalModelStatus();
    } catch(e) { console.error('Stop model error:', e); }
    modelStopping = false;
  }

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '—';
    if (bytes >= 1024*1024*1024) return (bytes / (1024*1024*1024)).toFixed(1) + ' GB';
    if (bytes >= 1024*1024) return (bytes / (1024*1024)).toFixed(1) + ' MB';
    return (bytes / 1024).toFixed(0) + ' KB';
  }
</script>

<!-- ═══ Setup Wizard Overlay ═══ -->
{#if showSetup}
<div class="overlay" style="z-index: 1000; display:flex;">
  <div class="modal setup-wizard">
    <div class="m-head" style="justify-content:center; border-bottom:none; padding-bottom:0;">
      <span class="m-title" style="font-size:18px; font-weight:700;">{t('wizardTitle')}</span>
    </div>
    <div class="m-body" style="padding:var(--sp-4) var(--sp-6);">
      <p class="m-desc" style="text-align:center; font-size:14px; margin-bottom:var(--sp-5);">{t('wizardDesc')}</p>
      
      <div class="m-section">
        <label for="setup-name" class="setting-label">{t('nameLabel')}</label>
        <input id="setup-name" type="text" bind:value={setupName} placeholder="E.g. Buğra Akdemir" class="setup-input"/>
      </div>
      
      <div class="m-section" style="margin-top:var(--sp-4);">
        <label for="setup-prompt" class="setting-label">{t('systemPromptLabel')}</label>
        <p class="m-desc" style="margin-bottom:4px;">{t('systemPromptDesc')}</p>
        <textarea id="setup-prompt" bind:value={setupPrompt} class="m-prompt" placeholder="You are Memo, a highly capable AI assistant…" rows="4"></textarea>
      </div>

      <div class="m-section" style="margin-top:var(--sp-5); background:var(--bg-app); border:1px solid var(--border-soft); border-radius:var(--r-md); padding:var(--sp-4);">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:var(--sp-3);">
          <span class="setting-label" style="font-size:13px; color:var(--text-muted);">Connection Diagnostics</span>
          <button class="lang-btn" style="background:var(--bg-hover);" on:click={checkSetupConnection} disabled={setupChecking}>{setupChecking ? '...' : t('refresh')}</button>
        </div>
        
        <div class="setup-check" class:ok={setupLMStatus}>
          <span class="check-icon">{setupLMStatus ? '✓' : '✗'}</span>
          <span>{t('checkLM')}</span>
        </div>
        <div class="setup-check" class:ok={setupModelStatus}>
          <span class="check-icon">{setupModelStatus ? '✓' : '✗'}</span>
          <span>{t('checkModels')}</span>
        </div>
        
        {#if !setupLMStatus || !setupModelStatus}
          <div style="margin-top:12px; font-size:12px; color:var(--red); line-height:1.4;">
            {t('lmStudioWarning')}
          </div>
        {/if}
      </div>

    </div>
    <div class="m-actions" style="padding:var(--sp-4) var(--sp-6); border-top:1px solid var(--text-dim); justify-content:flex-end; margin-top:0;">
      <button class="m-btn gold" on:click={finishSetup} disabled={!setupLMStatus}>{t('ready')}</button>
    </div>
  </div>
</div>
{/if}

<div class="app-container">
  <!-- NAV RAIL -->
  <div class="nav-rail">
    <div class="nav-rail-top">
      <button class="nav-rail-btn" class:active={currentView==='chat'} on:click={() => currentView='chat'} title="Chat">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>
      </button>
      {#if isDesktop}
      <button class="nav-rail-btn" class:active={currentView==='models'} on:click={openModelsTab} title="Models Store">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path></svg>
      </button>
      {/if}
    </div>
    <div class="nav-rail-bottom">
      <button class="nav-rail-btn" on:click={() => settingsOpen=true} title="Settings">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>
      </button>
    </div>
  </div>

  <div class="app-content">
  {#if currentView === 'chat'}

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
            
            {#if m.role === 'assistant' && m.stats}
              <div class="m-stats-wrap">
                <button class="m-stats-trigger" title="Generation stats">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>
                </button>
                <div class="m-stats-popover">
                  <div class="m-stat-row">
                    <span class="m-stat-label">{t('speed')}:</span>
                    <span class="m-stat-val">{(m.stats.tokens_per_second || 0).toFixed(2)} tok/s</span>
                  </div>
                  <div class="m-stat-row">
                    <span class="m-stat-label">{t('totalTokens')}:</span>
                    <span class="m-stat-val">{m.stats.completion_tokens}</span>
                  </div>
                  <div class="m-stat-row">
                    <span class="m-stat-label">{t('duration')}:</span>
                    <span class="m-stat-val">{(m.stats.total_duration_secs || 0).toFixed(2)}s</span>
                  </div>
                  {#if m.stats.stop_reason}
                  <div class="m-stat-row">
                    <span class="m-stat-label">{t('finishReason')}:</span>
                    <span class="m-stat-val">{m.stats.stop_reason}</span>
                  </div>
                  {/if}
                </div>
              </div>
            {/if}
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
              {#if m.isStreaming && !m.text}
                <div class="thinking-status">
                  <span class="thinking-text">{t('thinking')}</span>
                  <span class="pulse-dot"></span>
                </div>
              {:else}
                <div class="md">
                  {@html marked.parse(m.text || '')}
                  {#if m.isStreaming}<span class="cursor-blink">▊</span>{/if}
                </div>
              {/if}
            {:else}
              {m.text}
            {/if}
          </div>
        </div>
      {/each}

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

    {:else if currentView === 'models'}
       <div class="models-view-full shell">
          {#if !isLlamaInstalled}
             <div class="installer-card">
               <h2>Local Inference Engine Required</h2>
               <p>To use models locally without LM Studio, you need the llama.cpp engine. Click below to install it.</p>
               <button class="m-btn gold" on:click={startInstallation} disabled={isInstalling}>
                  {isInstalling ? 'Installing...' : (installFailed ? 'Retry Installation' : '1-Click Compile & Install')}
               </button>
               
               {#if isInstalling || installLog.length > 0}
                 <div id="install-log-box" class="install-log-box" style="{installFailed ? 'border-color: #ef4444;' : ''}">
                   {#each installLog as log}
                      <div>{log}</div>
                   {/each}
                 </div>
               {/if}
             </div>
          {:else}
             <div class="models-view-container">
        <div class="models-panel">
          <!-- Reinstall option -->
          {#if !isInstalling}
            <details class="reinstall-details">
              <summary class="reinstall-summary">Engine installed · Reinstall?</summary>
              <div style="padding: 0.75rem 0 0.25rem;">
                <button class="m-btn subtle" style="font-size:0.8rem;" on:click={startInstallation}>
                  {installFailed ? 'Retry Installation' : 'Recompile & Reinstall'}
                </button>
                <p style="color:#71717a; font-size:0.75rem; margin-top:0.5rem;">Use this if the engine crashes or shared libraries are missing.</p>
              </div>
            </details>
          {/if}
          {#if isInstalling || installLog.length > 0}
            <div id="install-log-box" class="install-log-box" style="{installFailed ? 'border-color: #ef4444;' : ''}">
              {#each installLog as log}
                <div>{log}</div>
              {/each}
            </div>
          {/if}

          <div class="gpu-badge" class:cpu={gpuInfo.type === 'cpu' || !gpuInfo.type} class:nvidia={gpuInfo.type === 'nvidia'} class:amd={gpuInfo.type === 'amd'}>
            {#if gpuInfo.type === 'cpu' || !gpuInfo.type}
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#eab308" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
              <span style="color:#eab308; font-weight: 500;">Models are running on CPU (Slow)</span>
            {:else}
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/></svg>
              <span>{gpuInfo.name}</span>
              {#if gpuInfo.vram_mb > 0}<span class="gpu-vram">{(gpuInfo.vram_mb / 1024).toFixed(0)} GB VRAM</span>{/if}
            {/if}
          </div>

          <!-- Running Model Status -->
          {#if localModelStatus.running}
            <div class="running-model-card">
              <div class="running-model-info">
                <span class="running-dot"></span>
                <div>
                  <div class="running-name">{localModelStatus.model_name}</div>
                  <div class="running-meta">Port {localModelStatus.port} · PID {localModelStatus.pid} · {localModelStatus.gpu.type.toUpperCase()}</div>
                </div>
              </div>
              <button class="m-btn danger" on:click={stopModel} disabled={modelStopping}>
                {modelStopping ? '...' : 'Stop'}
              </button>
            </div>
          {/if}

          <!-- Download Progress -->
          {#if downloadProgress.active}
            <div class="download-card">
              <div class="download-header">
                <span class="download-name">{downloadProgress.filename}</span>
                <button class="download-cancel" on:click={cancelDL}>✕</button>
              </div>
              <div class="download-bar-track">
                <div class="download-bar-fill" style="width: {downloadProgress.percent.toFixed(1)}%"></div>
              </div>
              <div class="download-stats">
                <span>{downloadProgress.percent.toFixed(1)}%</span>
                <span>{formatBytes(downloadProgress.downloaded)} / {formatBytes(downloadProgress.total_bytes)}</span>
                <span>{downloadProgress.speed}</span>
              </div>
            </div>
          {/if}

          <!-- Direct Model Entry -->
          <div class="model-entry-card" style="margin-bottom: 24px; background: var(--bg-panel); border: 1px solid var(--border-soft); border-radius: var(--r-md); padding: 20px;">
            <h3 style="margin-bottom: 8px;">{t('addModel')}</h3>
            <p class="m-desc" style="margin-bottom: 16px;">{t('modelDesc')}<br/>Example: <code style="background: var(--bg-app); padding: 2px 6px; border-radius: 4px; font-family: var(--mono); font-size: 13px;">google/gemma-3-4b-it</code></p>
            
            <div class="model-search-box">
              <label for="repo-id-input" class="sr-only">Repo ID</label>
              <input id="repo-id-input" type="text" bind:value={manualRepoID} placeholder={t('repoIdPlaceholder')} class="model-search-input" on:keydown={onSearchKey} />
              <button class="m-btn gold" on:click={fetchRepoFiles} disabled={modelSearching || !manualRepoID.trim()}>
                {modelSearching ? '…' : t('getFiles')}
              </button>
            </div>
          </div>

          <!-- Skeleton while searching -->
          {#if modelSearching}
            <div class="model-results">
              {#each Array(3) as _}
                <div class="model-result-card">
                  <div style="padding: 12px 16px; display:flex; flex-direction:column; gap:6px;">
                    <div class="skeleton" style="height:13px; width:60%; border-radius:6px;"></div>
                    <div class="skeleton" style="height:11px; width:30%; border-radius:6px;"></div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}

          <!-- Search Error -->
          {#if modelSearchError && !modelSearching}
            <div style="padding: 0.75rem 1rem; margin-bottom: 1rem; border-radius: 8px; background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.2); color: var(--red); font-size: 0.82rem;">
              {modelSearchError}
            </div>
          {/if}

          <!-- Model Files List (Result of manual entry) -->
          {#if modelFiles.length > 0 && !modelSearching}
            <div class="model-results-files" style="margin-top: 16px; margin-bottom: 32px;">
              <h4 class="section-title" style="margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                {manualRepoID}
              </h4>
              <div class="model-files-list" style="border: 1px solid var(--border-soft); border-radius: var(--r-md); background: white; overflow: hidden; box-shadow: var(--shadow-sm);">
                {#each modelFiles as file}
                  <div class="model-file-row" style="padding: 14px 16px; border-bottom: 1px solid var(--border-soft); display: flex; justify-content: space-between; align-items: center; gap: 12px; transition: background 150ms;" class:hover-bg={true}>
                    <div class="model-file-info" style="display: flex; flex-direction: column; gap: 4px; min-width: 0; flex: 1;">
                      <span class="model-file-name" style="font-weight: 600; font-size: 13.5px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--text-main);">{file.filename}</span>
                      <div style="display: flex; align-items: center; gap: 8px;">
                        <span class="model-file-size" style="font-size: 11.5px; color: var(--text-muted);">{formatBytes(file.size)}</span>
                        {#if gpuInfo && gpuInfo.vram_mb > 0}
                          {#if (file.size / (1024*1024)) > gpuInfo.vram_mb}
                            <span class="sys-req error" style="font-size: 10px; padding: 1px 6px; border-radius: 4px; background: rgba(239,68,68,0.08); color: var(--red);">{t('vramInsufficient')}</span>
                          {:else}
                            <span class="sys-req good" style="font-size: 10px; padding: 1px 6px; border-radius: 4px; background: rgba(81,181,118,0.08); color: var(--green);">{t('gpuCompatible')}</span>
                          {/if}
                        {/if}
                      </div>
                    </div>
                    <button class="m-btn gold" style="padding: 6px 12px; font-size: 12px; height: 32px; flex-shrink: 0;" on:click={() => startDownload(manualRepoID, file.filename)} disabled={downloadProgress.active}>
                      ↓ {t('download')}
                    </button>
                  </div>
                {/each}
              </div>
            </div>
          {/if}


          <!-- Local Models -->
          <div class="local-models-section">
            <h4 class="section-title">{t('downloadedModels')}</h4>
            {#if localModelLoading}
              <div class="mem-empty">Loading...</div>
            {:else if !localModels || localModels.length === 0}
              <div class="mem-empty">{t('noModels')}</div>
            {:else}
              {#each localModels as model}
                <div class="local-model-card" class:active-model={(localModelStatus.running && localModelStatus.model_path === model.path) || (embeddingModelStatus.running && embeddingModelStatus.model_path === model.path)}>
                  <div class="local-model-info">
                    <span class="local-model-name">
                      {model.filename}
                      {#if model.is_embedding}
                        <span style="font-size: 0.7rem; background: rgba(74,222,128,0.15); color: #4ade80; padding: 1px 6px; border-radius: 4px; margin-left: 6px;">embed</span>
                      {/if}
                    </span>
                    <span class="local-model-meta">{model.repo_id} · {formatBytes(model.size)}</span>
                  </div>
                  <div class="local-model-actions">
                    {#if localModelStatus.running && localModelStatus.model_path === model.path}
                      <span class="model-active-badge">● {t('running')}</span>
                    {:else if embeddingModelStatus.running && embeddingModelStatus.model_path === model.path}
                      <span class="model-active-badge" style="color: #4ade80;">● {t('embedding')}</span>
                    {:else}
                      <button class="m-btn small-start-btn" on:click={() => openModelConfig(model)} disabled={modelStarting === model.path || localModelStatus.running || model.is_embedding}>
                        {modelStarting === model.path ? t('loading') : '▶ ' + t('start')}
                      </button>
                    {/if}
                    <button class="model-del-btn" on:click={() => deleteModel(model.path)} disabled={(localModelStatus.running && localModelStatus.model_path === model.path) || (embeddingModelStatus.running && embeddingModelStatus.model_path === model.path)} title={t('delete')}>🗑</button>
                  </div>
                </div>
              {/each}
            {/if}
          </div>
        </div>
             </div>
          {/if}
       </div>
    {/if}
  </div> <!-- end app-content -->
</div> <!-- end app-container -->

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
    <!-- ═══ Settings Modal ═══ -->
      <button class="m-tab" class:active={settingsTab==='about'} on:click={() => settingsTab='about'}>{t('about')}</button>
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
          <div class="setting-row">
            <span class="setting-label">Setup Wizard</span>
            <button class="m-btn" on:click={() => { localStorage.removeItem('memo_setup_complete'); showSetup=true; checkSetupConnection(); settingsOpen=false; }}>{t('resetSetup')}</button>
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
                <button class="mem-x" on:click={() => delMem(f.path)} aria-label="Delete memory file">×</button>
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
              <button class="toggle-switch" class:on={remoteEnabled} on:click={() => remoteEnabled = !remoteEnabled} aria-pressed={remoteEnabled} aria-label="Toggle remote access">
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
            <button class="m-btn gold" on:click={saveRemoteAccess} disabled={remoteSaving} aria-label="Save remote access settings">
              {remoteSaving ? '…' : 'Save & Apply'}
            </button>
          </div>
        </div>
      {:else if settingsTab === 'about'}
        <div class="m-section about-section">
          <h3 style="margin: 0; display: flex; align-items: center; justify-content: space-between;">
            Memo AI
            <span style="font-size: 11px; opacity: 0.5; font-weight: normal;">{appVersion}</span>
          </h3>
          <p class="m-desc">{t('aboutDev')}: <strong>Buğra Akdemir</strong></p>
          <div class="about-card mt-3">
            <h4>{t('aboutVisionTitle')}</h4>
            <p>{t('aboutVisionText')}</p>
          </div>
          <div class="about-card mt-3">
            <h4>{t('aboutPrivacyTitle')}</h4>
            <p>{t('aboutPrivacyText')}</p>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>
{/if}

<!-- ═══ Model Config Overlay ═══ -->
{#if configModalOpen && modelToStart}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="modal-overlay" on:click={() => configModalOpen = false}>
    <div class="modal-content run-config-modal" on:click|stopPropagation>
      <div class="modal-header">
        <h2 class="modal-title">Configure & Load Model</h2>
        <button class="modal-close" on:click={() => configModalOpen = false} aria-label="Close">×</button>
      </div>
      <div class="modal-body setup-body">
        <div class="model-info-box">
          <p class="model-info-filename">
            {modelToStart.filename}
          </p>
          <p class="model-info-size">Size: {formatBytes(modelToStart.size)}</p>
        </div>

        <div class="settings-form">
          <div class="form-group">
            <label for="configCtx">Context Size (Tokens)</label>
            <input id="configCtx" type="number" class="w-input" bind:value={configCtxSize} min="512" max="128000" step="512" />
            <span class="form-hint">Higher context uses more RAM/VRAM. (Default: 4096)</span>
          </div>
          <div class="form-group">
            <label for="configGpu">GPU Layers (-1 for Max)</label>
            <input id="configGpu" type="number" class="w-input" bind:value={configGpuLayers} min="-1" max="100" />
            <span class="form-hint">If your VRAM gets exhausted, lower this number. (0 = CPU only)</span>
          </div>
          <div class="form-group">
            <label for="configPort">Host Port</label>
            <input id="configPort" type="number" class="w-input" bind:value={configPort} min="1024" max="65535" />
          </div>
        </div>

        <!-- Embedding Model Suggestion -->
        {#if getAvailableEmbedModels().length > 0}
          <div class="embed-suggestion-box">
            <label class="embed-toggle">
              <input type="checkbox" bind:checked={embedModelEnabled} />
              <span class="embed-toggle-text">Start Embedding Model</span>
            </label>
            <p class="embed-desc">
              An embedding model improves memory search accuracy. It runs alongside the chat model using minimal resources.
            </p>
            {#if embedModelEnabled}
              <select bind:value={selectedEmbedModel} class="embed-select">
                {#each getAvailableEmbedModels() as em}
                  <option value={em}>{em.filename} ({formatBytes(em.size)})</option>
                {/each}
              </select>
            {/if}
          </div>
        {/if}
      </div>
      <div class="modal-actions">
        <button class="m-btn" on:click={() => configModalOpen = false}>Cancel</button>
        <button class="m-btn gold" on:click={confirmStartModel}>Run Engine</button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* ═══════════════════════════════════
     APP CONTAINER & NAV RAIL
  ═══════════════════════════════════ */
  :global(body) { margin: 0; padding: 0; background: var(--bg-body); }
  .app-container {
    display: flex;
    height: 100dvh;
    width: 100vw;
    overflow: hidden;
  }
  .nav-rail {
    width: 60px;
    background: var(--bg-app);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 16px 0;
    flex-shrink: 0;
    z-index: 10;
  }
  .nav-rail-top {
    display: flex;
    flex-direction: column;
    gap: 16px;
    flex: 1;
  }
  .nav-rail-bottom {
    margin-top: auto;
  }
  .nav-rail-btn {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    transition: background 0.2s ease, border-color 0.2s ease, transform 0.2s ease;
    border: none;
    background: transparent;
    cursor: pointer;
  }
  .nav-rail-btn:hover {
    color: var(--text-main);
    background: var(--bg-hover);
  }
  .nav-rail-btn.active {
    color: var(--accent);
    background: var(--accent-muted);
  }
  .app-content {
    flex: 1;
    display: flex;
    min-width: 0;
    position: relative;
    border-left: 1px solid var(--border-soft);
  }
  
  /* OVERRIDE EXISTING SHELL TO FILL PARENT */
  .app-content > .shell {
    width: 100%;
  }

  /* MODELS VIEW FULL */
  .models-view-full {
    width: 100%;
    height: 100%;
    background: var(--bg-app);
    display: flex;
    flex-direction: column;
    align-items: center;
    overflow-y: auto;
    padding: 32px 0;
  }
  .models-view-container {
    width: 100%;
    max-width: 900px;
    padding: 0 32px;
    margin: 0 auto;
  }
  
  /* INSTALLER UI */
  .installer-card {
    background: var(--bg-panel);
    border: 1px solid var(--border-soft);
    border-radius: var(--r-lg);
    padding: 40px;
    width: 100%;
    max-width: 600px;
    text-align: center;
    margin-top: 40px;
  }
  .installer-card h2 { margin-top: 0; color: var(--text-main); fill: var(--text-main); margin-bottom:12px; }
  .installer-card p { color: var(--text-muted); margin-bottom: 32px; font-size: 15px; }
  .install-log-box {
    margin-top: 32px;
    background: #111;
    border-radius: var(--r-md);
    padding: 16px;
    color: #4ade80;
    font-family: monospace;
    font-size: 12px;
    height: 300px;
    overflow-y: auto;
    text-align: left;
    white-space: pre-wrap;
    border: 1px solid #333;
    box-shadow: inset 0 2px 4px rgba(0,0,0,0.5);
  }

  /* ═══════════════════════════════════
     MODELS PANEL CSS
  ═══════════════════════════════════ */
  .models-panel {
    display: flex;
    flex-direction: column;
    gap: 24px;
    width: 100%;
    margin-bottom: 40px;
  }
  .gpu-badge {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    background: var(--bg-hover);
    border-radius: var(--r-md);
    font-size: 13px;
    color: var(--text-muted);
    align-self: flex-start;
    border: 1px solid var(--border-soft);
  }
  .gpu-badge.nvidia { border-color: #4ade80; color: #4ade80; background: rgba(74, 222, 128, 0.05); }
  .gpu-badge.amd { border-color: #f97316; color: #f97316; background: rgba(249, 115, 22, 0.05); }
  .gpu-badge.cpu { border-color: #eab308; color: #eab308; background: rgba(234, 179, 8, 0.05); }
  .gpu-vram {
    opacity: 0.7;
    margin-left: 4px;
  }
  .model-search-box {
    display: flex;
    gap: 12px;
  }
  .model-search-input {
    flex: 1;
    background: var(--bg-panel);
    border: 1px solid var(--border-soft);
    padding: 12px 16px;
    border-radius: var(--r-md);
    color: var(--text-main);
    font-size: 14px;
    outline: none;
    transition: border-color 0.2s;
  }
  .model-search-input:focus {
    border-color: var(--accent);
  }
  /* intentionally removed — canonical rules live in the second CSS block below */
  .sys-req {
    font-size: 0.7rem;
    padding: 2px 6px;
    border-radius: 4px;
    white-space: nowrap;
    font-weight: 500;
  }
  .sys-req.good { background: rgba(74, 222, 128, 0.1); color: #4ade80; }
  .sys-req.warn { background: rgba(250, 204, 21, 0.1); color: #facc15; }
  .sys-req.error { background: rgba(248, 113, 113, 0.1); color: #f87171; }

  .small-dl-btn {
    padding: 0.35rem 0.6rem;
    font-size: 0.75rem;
    background: rgba(255,255,255,0.05);
  }

  .small-start-btn {
    padding: 0.4rem 0.8rem;
    font-size: 0.75rem;
    background: rgba(74, 222, 128, 0.15);
    color: #4ade80;
    border: 1px solid rgba(74, 222, 128, 0.3);
  }
  .small-start-btn:hover:not(:disabled) {
    background: rgba(74, 222, 128, 0.25);
  }

  .model-active-badge {
    font-size: 0.75rem;
    color: #4ade80;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.35rem 0.6rem;
    background: rgba(74, 222, 128, 0.1);
    border-radius: 6px;
  }

  .active-model {
    border-color: rgba(74, 222, 128, 0.4) !important;
    background: rgba(74, 222, 128, 0.05) !important;
    box-shadow: 0 0 15px rgba(74, 222, 128, 0.1);
  }

  .local-models-section {
    margin-top: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .local-model-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px;
    background: var(--bg-panel);
    border: 1px solid var(--border-soft);
    border-radius: var(--r-md);
  }
  .local-model-card.active-model {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }
  .local-model-info { display: flex; flex-direction: column; gap: 4px; }
  .local-model-name { color: var(--text-main); font-weight: 500; font-size: 14.5px; }
  .local-model-meta { color: var(--text-dim); font-size: 12.5px; }
  .local-model-actions { display: flex; align-items: center; gap: 12px; }

  /* THINKING INDICATOR */
  .thinking-status {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    color: var(--text-dim);
    font-style: italic;
    font-size: 0.9rem;
  }
  .thinking-text {
    opacity: 0.8;
  }
  .pulse-dot {
    width: 6px;
    height: 6px;
    background: var(--accent);
    border-radius: 50%;
    animation: thinking-pulse 1.4s infinite ease-in-out;
  }
  @keyframes thinking-pulse {
    0%, 100% { transform: scale(0.6); opacity: 0.4; }
    50% { transform: scale(1.1); opacity: 1; }
  }

  .running-model-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px;
    background: rgba(163, 190, 140, 0.05);
    border: 1px solid rgba(163, 190, 140, 0.2);
    border-radius: var(--r-md);
  }
  .running-model-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .running-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent);
    animation: pulse 2s infinite;
  }
  @keyframes pulse {
    0% { transform: scale(1); opacity: 1; }
    50% { transform: scale(1.1); opacity: 0.7; }
    100% { transform: scale(1); opacity: 1; }
  }
  .running-name { color: var(--text-main); font-weight: 500; margin-bottom: 2px; }
  .running-meta { color: var(--accent); font-size: 12px; opacity: 0.9; }

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
    position: relative;
  }
  .m-stats-wrap {
    position: relative;
    display: flex;
    align-items: center;
    margin-left: 2px;
  }
  .m-stats-trigger {
    background: none;
    border: none;
    padding: 4px;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    opacity: 0.4;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    cursor: pointer;
    border-radius: 6px;
  }
  .m-stats-wrap:hover .m-stats-trigger {
    opacity: 1;
    color: var(--accent);
    background: var(--bg-element);
    box-shadow: var(--shadow-sm);
  }
  .m-stats-popover {
    position: absolute;
    top: 28px;
    left: 0;
    background: var(--bg-panel);
    border: 1px solid var(--border-soft);
    border-radius: var(--r-md);
    padding: 10px 14px;
    box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.2);
    display: flex;
    flex-direction: column;
    gap: 6px;
    z-index: 1000;
    opacity: 0;
    visibility: hidden;
    transform: translateY(-8px);
    transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
    min-width: 160px;
    backdrop-filter: blur(12px) saturate(180%);
    pointer-events: none;
  }
  .m-stats-wrap:hover .m-stats-popover {
    opacity: 1;
    visibility: visible;
    transform: translateY(0);
    pointer-events: auto;
  }
  .m-stat-row {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    white-space: nowrap;
    border-bottom: 1px solid rgba(0,0,0,0.03);
    padding-bottom: 2px;
  }
  .m-stat-row:last-child { border-bottom: none; }
  .m-stat-label {
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 500;
  }
  .m-stat-val {
    color: var(--text-main);
    font-weight: 700;
    font-size: 11px;
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
  
  .about-card { background: var(--bg-element); padding: var(--sp-4); border-radius: var(--r-md); margin-top: var(--sp-3); }
  .about-card h4 { margin: 0 0 var(--sp-2) 0; font-size: 13px; color: var(--text-main); }
  .about-card p { font-size: 13px; color: var(--text-muted); line-height: 1.6; margin: 0; }
  
  .setup-wizard { padding: 0; max-width: 540px; }
  .setup-input { width: 100%; border: 1px solid var(--border-soft); background: var(--bg-app); color: var(--text-main); padding: var(--sp-3); border-radius: var(--r-md); font-size: 14px; margin-top: 4px; }
  .setup-input:focus { border-color: var(--accent); }
  .setup-check { display: flex; align-items: center; gap: var(--sp-2); padding: var(--sp-2) 0; color: var(--text-muted); font-size: 13px; }
  .setup-check.ok { color: var(--green); }
  .check-icon { width: 24px; height: 24px; border-radius: 50%; background: var(--border-soft); display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: bold; }
  .setup-check.ok .check-icon { background: rgba(34, 197, 94, 0.15); color: var(--green); }
  
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

  .models-panel {
    display: flex; flex-direction: column; gap: 12px;
  }

  /* GPU Badge */
  .gpu-badge {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 14px;
    border-radius: var(--r-md);
    background: rgba(100, 100, 120, 0.1);
    border: 1px solid rgba(100, 100, 120, 0.2);
    font-size: 12px; color: var(--text-secondary);
  }
  .gpu-badge.nvidia {
    background: rgba(118, 185, 0, 0.08);
    border-color: rgba(118, 185, 0, 0.25);
    color: #76b900;
  }
  .gpu-badge.amd {
    background: rgba(237, 28, 36, 0.08);
    border-color: rgba(237, 28, 36, 0.2);
    color: #ed1c24;
  }
  .gpu-vram {
    margin-left: auto;
    font-size: 11px;
    opacity: 0.8;
    font-weight: 600;
  }

  /* Running Model Card */
  .running-model-card {
    display: flex; align-items: center; justify-content: space-between;
    padding: 12px 16px;
    border-radius: var(--r-md);
    background: linear-gradient(135deg, rgba(16, 185, 129, 0.08), rgba(16, 185, 129, 0.03));
    border: 1px solid rgba(16, 185, 129, 0.25);
  }
  .running-model-info { display: flex; align-items: center; gap: 10px; }
  .running-dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: #10b981;
    box-shadow: 0 0 8px rgba(16, 185, 129, 0.6);
    animation: pulse-glow 2s ease-in-out infinite;
  }
  @keyframes pulse-glow {
    0%, 100% { box-shadow: 0 0 6px rgba(16, 185, 129, 0.4); }
    50% { box-shadow: 0 0 14px rgba(16, 185, 129, 0.8); }
  }
  .running-name { font-size: 13px; font-weight: 600; color: var(--text-main); }
  .running-meta { font-size: 11px; color: var(--text-muted); margin-top: 2px; }

  /* Download Card */
  .download-card {
    padding: 12px 16px;
    border-radius: var(--r-md);
    background: rgba(59, 130, 246, 0.06);
    border: 1px solid rgba(59, 130, 246, 0.2);
  }
  .download-header {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 8px;
  }
  .download-name {
    font-size: 12px; font-weight: 500; color: var(--text-main);
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
    flex: 1; margin-right: 8px;
  }
  .download-cancel {
    width: 22px; height: 22px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px; font-size: 12px;
    color: var(--text-muted);
  }
  .download-cancel:hover { background: rgba(239, 68, 68, 0.15); color: var(--red); }
  .download-bar-track {
    width: 100%; height: 6px;
    border-radius: 3px;
    background: rgba(59, 130, 246, 0.1);
    overflow: hidden;
  }
  .download-bar-fill {
    height: 100%;
    border-radius: 3px;
    background: linear-gradient(90deg, #3b82f6, #60a5fa);
    transition: width 300ms ease;
    box-shadow: 0 0 8px rgba(59, 130, 246, 0.4);
  }
  .download-stats {
    display: flex; justify-content: space-between;
    margin-top: 6px; font-size: 11px; color: var(--text-muted);
  }

  /* Search Box */
  .model-search-box {
    display: flex; gap: 8px;
  }
  .model-search-input {
    flex: 1;
    padding: 8px 12px;
    border-radius: var(--r-md);
    border: 1px solid var(--border-soft);
    background: var(--bg-app);
    color: var(--text-main);
    font-size: 13px;
    outline: none;
  }
  .model-search-input:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgba(168, 85, 247, 0.15);
  }
  .model-search-input::placeholder { color: var(--text-muted); }

  /* ═══ SKELETON LOADER ═══ */
  .skeleton {
    background: linear-gradient(90deg, var(--bg-element) 25%, var(--bg-hover) 50%, var(--bg-element) 75%);
    background-size: 200% 100%;
    animation: shimmer 1.4s infinite;
  }
  @keyframes shimmer {
    0% { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }

  /* ═══ SEARCH RESULTS (scrollable) ═══ */
  .model-results {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 8px;
    max-height: 440px;
    overflow-y: auto;
    background: var(--bg-panel);
    border: 1px solid var(--border-soft);
    border-radius: var(--r-md);
    margin-top: 8px;
  }
  .model-results::-webkit-scrollbar { width: 5px; }
  .model-results::-webkit-scrollbar-track { background: transparent; }
  .model-results::-webkit-scrollbar-thumb { background: var(--text-dim); border-radius: 99px; }

  .model-result-card {
    background: white;
    border: 1px solid var(--border-soft);
    border-radius: var(--r-sm);
    overflow: hidden;
    transition: transform 200ms ease, border-color 200ms ease, box-shadow 200ms ease;
  }
  .model-result-card:hover {
    border-color: var(--text-dim);
    box-shadow: var(--shadow-sm);
    transform: translateY(-1px);
  }


  /* Local Models */
  .local-models-section { margin-top: 4px; }
  .section-title {
    font-size: 13px; font-weight: 600;
    color: var(--text-muted);
    margin: 0 0 8px 0;
    padding-bottom: 6px;
    border-bottom: 1px solid var(--border-soft);
  }
  .local-model-card {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 14px;
    border-radius: var(--r-md);
    border: 1px solid var(--border-soft);
    background: var(--bg-app);
    margin-bottom: 4px;
    transition: border-color 200ms;
  }
  .local-model-card:hover { border-color: var(--accent); }
  .local-model-card.active-model {
    border-color: rgba(16, 185, 129, 0.4);
    background: rgba(16, 185, 129, 0.04);
  }
  .local-model-info { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0; }
  .local-model-name {
    font-size: 12px; font-weight: 500; font-family: var(--mono);
    color: var(--text-main);
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .local-model-meta { font-size: 11px; color: var(--text-muted); }
  .local-model-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }

  .small-start-btn {
    padding: 4px 10px !important; font-size: 11px !important;
    background: rgba(16, 185, 129, 0.1) !important;
    color: #10b981 !important;
    border: 1px solid rgba(16, 185, 129, 0.2) !important;
  }
  .small-start-btn:hover:not(:disabled) {
    background: rgba(16, 185, 129, 0.2) !important;
  }
  .small-start-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .model-active-badge {
    font-size: 11px; font-weight: 600;
    color: #10b981;
    padding: 3px 8px;
    border-radius: 999px;
    background: rgba(16, 185, 129, 0.1);
    animation: pulse-glow 2s ease-in-out infinite;
  }
  .model-del-btn {
    width: 26px; height: 26px;
    display: flex; align-items: center; justify-content: center;
    border-radius: 4px; font-size: 13px;
    color: var(--text-muted); opacity: 0.5;
    transition: all 150ms;
  }
  .model-del-btn:hover:not(:disabled) { opacity: 1; background: rgba(239, 68, 68, 0.12); color: var(--red); }
  .model-del-btn:disabled { opacity: 0.2; cursor: not-allowed; }
  /* ═══ START CONFIG MODAL & SEARCH UI FIXES ═══ */
  .modal-overlay {
    position: fixed; top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    display: flex; align-items: center; justify-content: center;
    z-index: 9999;
    animation: fadeIn 0.15s ease-out;
  }
  .modal-content {
    background: var(--bg-panel);
    border: 1px solid var(--border-soft);
    box-shadow: 0 24px 64px rgba(0,0,0,0.25);
    border-radius: var(--r-lg);
    width: 480px; max-width: 90vw;
    display: flex; flex-direction: column;
    animation: modalPop 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }
  @keyframes modalPop {
    from { opacity: 0; transform: scale(0.96) translateY(10px); }
    to { opacity: 1; transform: scale(1) translateY(0); }
  }
  .modal-header {
    padding: var(--sp-4) var(--sp-5);
    border-bottom: 1px solid var(--border-soft);
    display: flex; justify-content: space-between; align-items: center;
  }
  .modal-title { font-size: 13px; font-weight: 700; color: var(--text-main); margin:0; letter-spacing: 0.3px; }
  .modal-close { background: none; border: none; color: var(--text-muted); cursor: pointer; font-size: 1.25rem; transition: color 0.15s; }
  .modal-close:hover { color: var(--text-main); }
  .modal-body { padding: 1.25rem; display: flex; flex-direction: column; gap: 1rem; }
  .modal-actions {
    padding: var(--sp-4) var(--sp-5);
    border-top: 1px solid var(--border-soft);
    display: flex; justify-content: flex-end; gap: var(--sp-2);
  }

  .model-info-box {
    background: var(--bg-element);
    padding: var(--sp-3) var(--sp-4);
    border-radius: var(--r-md);
    border: 1px solid var(--border-soft);
    margin-bottom: var(--sp-4);
  }
  .model-info-filename {
    font-family: var(--mono);
    font-size: 12px;
    color: var(--accent);
    margin: 0 0 4px 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 600;
  }
  .model-info-size { font-size: 11px; color: var(--text-muted); margin: 0; }

  /* ═══ CONFIG DIALOG FORM ═══ */
  .settings-form { display: flex; flex-direction: column; gap: var(--sp-4); }
  .form-group { display: flex; flex-direction: column; gap: 6px; }
  .form-group label { color: var(--text-main); font-size: 13px; font-weight: 600; }
  .w-input {
    background: var(--bg-app);
    border: 1px solid var(--border-soft);
    border-radius: var(--r-md);
    padding: 10px 12px;
    color: var(--text-main);
    font-size: 14px;
    outline: none;
    transition: all 0.15s ease;
    -moz-appearance: textfield;
    appearance: textfield;
  }
  .w-input:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 2px rgba(214, 188, 148, 0.15);
  }
  .form-hint { color: var(--text-muted); font-size: 11px; line-height: 1.4; }

  /* Embedding suggestion */
  .embed-suggestion-box {
    margin-top: var(--sp-4);
    background: rgba(16, 185, 129, 0.05);
    border: 1px solid rgba(16, 185, 129, 0.2);
    border-radius: var(--r-md);
    padding: 14px 16px;
  }
  .embed-toggle { display: flex; align-items: center; gap: 10px; cursor: pointer; margin-bottom: 6px; }
  .embed-toggle input { accent-color: var(--green); width: 16px; height: 16px; }
  .embed-toggle-text { font-size: 13px; font-weight: 700; color: var(--green); }
  .embed-desc { font-size: 11px; color: var(--text-muted); margin: 0 0 10px 0; line-height: 1.5; }
  .embed-select {
    width: 100%;
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--bg-app);
    border: 1px solid var(--border-soft);
    color: var(--text-main);
    font-size: 13px;
    outline: none;
  }
  .embed-select:focus { border-color: var(--accent); }
  .setup-body { padding: 1.25rem; }

  /* ═══ SEARCH MODELS UI ═══ */
</style>

