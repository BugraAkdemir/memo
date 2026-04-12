// Detects if running inside Wails or standalone browser
// and provides a unified API for both environments.

const isWails = typeof window !== 'undefined' && window.runtime !== undefined;

// REST API base - when accessed from browser, use same origin
const API_BASE = '/api';

async function restPost(endpoint, body = {}) {
  const resp = await fetch(`${API_BASE}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

async function restGet(endpoint) {
  const resp = await fetch(`${API_BASE}${endpoint}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json();
}

// Lazy-load Wails bindings only when in Wails environment
let wailsApp = null;
async function getWailsApp() {
  if (!wailsApp) {
    wailsApp = await import('../../wailsjs/go/main/App.js');
  }
  return wailsApp;
}

// ─── Exported API functions ─────────────────────────────────────

export async function SendMessage(msg) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessage(msg);
  }
  const data = await restPost('/send', { message: msg });
  return data.reply;
}

export async function SendMessageWithImage(msg, payload) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageWithImage(msg, payload);
  }
  if (payload instanceof File) {
    const formData = new FormData();
    formData.append('message', msg);
    formData.append('file', payload);
    const resp = await fetch(`${API_BASE}/send_file`, { method: 'POST', body: formData });
    if (!resp.ok) throw new Error(await resp.text());
    const data = await resp.json();
    return data.reply;
  }
  const data = await restPost('/send', { message: msg });
  return data.reply;
}

export async function SendMessageWithFile(msg, payload) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageWithFile(msg, payload);
  }
  if (payload instanceof File) {
    const formData = new FormData();
    formData.append('message', msg);
    formData.append('file', payload);
    const resp = await fetch(`${API_BASE}/send_file`, { method: 'POST', body: formData });
    if (!resp.ok) throw new Error(await resp.text());
    const data = await resp.json();
    return data.reply;
  }
  const data = await restPost('/send', { message: msg });
  return data.reply;
}

export async function ToggleIncognito(enabled) {
  if (isWails) {
    const w = await getWailsApp();
    return w.ToggleIncognito(enabled);
  }
  await restPost('/incognito', { enabled });
}

export async function NewChat() {
  if (isWails) {
    const w = await getWailsApp();
    return w.NewChat();
  }
  const data = await restPost('/chats/new');
  return data.id;
}

export async function ListChats() {
  if (isWails) {
    const w = await getWailsApp();
    return w.ListChats();
  }
  return restGet('/chats');
}

export async function SwitchChat(id) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SwitchChat(id);
  }
  await restPost('/chats/switch', { id });
}

export async function DeleteChat(id) {
  if (isWails) {
    const w = await getWailsApp();
    return w.DeleteChat(id);
  }
  await restPost('/chats/delete', { id });
}

export async function GetActiveMessages() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetActiveMessages();
  }
  return restGet('/messages');
}

export async function GetActiveChatID() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetActiveChatID();
  }
  const data = await restGet('/chats/active');
  return data.id;
}

export async function CheckConnection() {
  if (isWails) {
    const w = await getWailsApp();
    return w.CheckConnection();
  }
  const data = await restGet('/status');
  return data.connection;
}

export async function GetMemoryCount() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetMemoryCount();
  }
  const data = await restGet('/status');
  return data.memory_count;
}

export async function SelectImage() {
  if (isWails) {
    const w = await getWailsApp();
    return w.SelectImage();
  }
  return ''; // Not available on web
}

export async function SelectFile() {
  if (isWails) {
    const w = await getWailsApp();
    return w.SelectFile();
  }
  return ''; // Not available on web
}

export async function GetSystemPrompt() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetSystemPrompt();
  }
  return '';
}

export async function SetSystemPrompt(prompt) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SetSystemPrompt(prompt);
  }
}

export async function ResetSystemPrompt() {
  if (isWails) {
    const w = await getWailsApp();
    return w.ResetSystemPrompt();
  }
}

export async function GetIncognitoPrompt() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetIncognitoPrompt();
  }
  return '';
}

export async function SetIncognitoPrompt(prompt) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SetIncognitoPrompt(prompt);
  }
}

export async function ClearAllMemory() {
  if (isWails) {
    const w = await getWailsApp();
    return w.ClearAllMemory();
  }
}

export async function ListMemoryFiles() {
  if (isWails) {
    const w = await getWailsApp();
    return w.ListMemoryFiles();
  }
  return [];
}

export async function DeleteMemoryFile(path) {
  if (isWails) {
    const w = await getWailsApp();
    return w.DeleteMemoryFile(path);
  }
}

export async function GetImageBase64(path) {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetImageBase64(path);
  }
  return '';
}

let _mediaRecorder = null;
let _audioChunks = [];
let _mediaStream = null;

export async function StartRecording() {
  if (isWails) {
    const w = await getWailsApp();
    return w.StartRecording();
  }
  _audioChunks = [];
  _mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
  _mediaRecorder = new MediaRecorder(_mediaStream, {
    mimeType: MediaRecorder.isTypeSupported('audio/webm;codecs=opus') ? 'audio/webm;codecs=opus' : 'audio/webm'
  });
  _mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) _audioChunks.push(e.data); };
  _mediaRecorder.start(100);
}

export async function StopRecordingAndTranscribe() {
  if (isWails) {
    const w = await getWailsApp();
    return w.StopRecordingAndTranscribe();
  }
  if (!_mediaRecorder || _mediaRecorder.state !== 'recording') throw new Error('Not recording');
  return new Promise((resolve, reject) => {
    _mediaRecorder.onstop = async () => {
      if (_mediaStream) { _mediaStream.getTracks().forEach(t => t.stop()); _mediaStream = null; }
      const blob = new Blob(_audioChunks, { type: _mediaRecorder.mimeType });
      _audioChunks = [];
      _mediaRecorder = null;
      try {
        const resp = await fetch(`${API_BASE}/transcribe`, { method: 'POST', body: blob });
        if (!resp.ok) throw new Error(await resp.text());
        const data = await resp.json();
        resolve(data.text || '');
      } catch (e) { reject(e); }
    };
    _mediaRecorder.stop();
  });
}

export async function GetRemoteAccessStatus() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetRemoteAccessStatus();
  }
  return { enabled: false, port: 8080, running: false, addresses: [] };
}

export async function SetRemoteAccess(enabled, port) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SetRemoteAccess(enabled, port);
  }
}

export async function GetVersion() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetVersion();
  }
  return 'V1.0.0-Browser';
}

// ─── Cloud Sync ──────────────────────────────────────────────────

export async function CheckAuth() {
  if (!isWails) return false;
  const w = await getWailsApp();
  if (typeof w.CheckAuth === 'function') return w.CheckAuth();
  if (typeof w.CheckSyncAuth === 'function') return w.CheckSyncAuth();
  return false;
}

export async function StartSyncAuth() {
  if (!isWails) return '';
  const w = await getWailsApp();
  if (typeof w.StartSyncAuth === 'function') return w.StartSyncAuth();
  throw new Error('Cloud sync auth is not available');
}

export async function DisconnectSync() {
  if (!isWails) return;
  const w = await getWailsApp();
  if (typeof w.DisconnectSync === 'function') return w.DisconnectSync();
  throw new Error('DisconnectSync is not available');
}

export async function TriggerSync() {
  if (!isWails) return;
  const w = await getWailsApp();
  if (typeof w.TriggerSync === 'function') return w.TriggerSync();
  throw new Error('Cloud sync trigger is not available');
}

export async function GetSyncAccount() {
  if (!isWails) return { authenticated: false, name: '', email: '' };
  const w = await getWailsApp();
  if (typeof w.GetSyncAccount === 'function') {
    return w.GetSyncAccount();
  }
  return { authenticated: false, name: '', email: '' };
}

export async function GetSyncSettings() {
  if (!isWails) return { enabled: false, client_id: '', client_secret: '', passphrase: '', token_path: './data/sync_token.json', interval_messages: 50 };
  const w = await getWailsApp();
  if (typeof w.GetSyncSettings === 'function') return w.GetSyncSettings();
  return { enabled: false, client_id: '', client_secret: '', passphrase: '', token_path: './data/sync_token.json', interval_messages: 50 };
}

export async function UpdateSyncSettings(enabled, clientID, clientSecret, passphrase, tokenPath, intervalMessages) {
  if (!isWails) return;
  const w = await getWailsApp();
  if (typeof w.UpdateSyncSettings === 'function') {
    return w.UpdateSyncSettings(enabled, clientID, clientSecret, passphrase, tokenPath, intervalMessages);
  }
  throw new Error('Sync settings update is not available');
}

export async function PullSync() {
  if (!isWails) return;
  const w = await getWailsApp();
  if (typeof w.PullSync === 'function') return w.PullSync();
  throw new Error('Sync pull is not available');
}

export async function SyncNow() {
  if (!isWails) return;
  const w = await getWailsApp();
  if (typeof w.SyncNow === 'function') return w.SyncNow();
  throw new Error('Full sync is not available');
}

// Utility: check if running in Wails
export function isWailsEnvironment() {
  return isWails;
}

// ─── Model Store ─────────────────────────────────────────────────

export async function SearchModels(query) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SearchModels(query);
  }
  return [];
}

export async function GetModelFiles(repoID) {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetModelFiles(repoID);
  }
  return [];
}

export async function DownloadModel(repoID, filename) {
  if (isWails) {
    const w = await getWailsApp();
    return w.DownloadModel(repoID, filename);
  }
}

export async function GetDownloadProgress() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetDownloadProgress();
  }
  return { active: false, percent: 0, speed: '', downloaded: 0, total_bytes: 0 };
}

export async function CancelDownload() {
  if (isWails) {
    const w = await getWailsApp();
    return w.CancelDownload();
  }
}

export async function ListLocalModels() {
  if (isWails) {
    const w = await getWailsApp();
    return w.ListLocalModels();
  }
  return [];
}

export async function DeleteLocalModel(path) {
  if (isWails) {
    const w = await getWailsApp();
    return w.DeleteLocalModel(path);
  }
}

// ─── llama-server Management ────────────────────────────────────

export async function StartLocalModel(modelPath, ctxSize = 4096, port = 8081, gpuLayers = -1) {
  if (isWails) {
    const w = await getWailsApp();
    return w.StartLocalModel(modelPath, ctxSize, port, gpuLayers);
  }
}

export async function StopLocalModel() {
  if (isWails) {
    const w = await getWailsApp();
    return w.StopLocalModel();
  }
}

export async function GetLocalModelStatus() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetLocalModelStatus();
  }
  return { running: false, model_name: '', port: 0, gpu: { type: 'cpu', name: 'N/A' } };
}

export async function StartEmbeddingModel(modelPath, gpuLayers = -1) {
  if (isWails) {
    const w = await getWailsApp();
    return w.StartEmbeddingModel(modelPath, gpuLayers);
  }
}

export async function StopEmbeddingModel() {
  if (isWails) {
    const w = await getWailsApp();
    return w.StopEmbeddingModel();
  }
}

export async function GetEmbeddingModelStatus() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetEmbeddingModelStatus();
  }
  return { running: false, model_name: '', port: 0, gpu: { type: 'cpu', name: 'N/A' } };
}

export async function DetectGPU() {
  if (isWails) {
    const w = await getWailsApp();
    return w.DetectGPU();
  }
  return { type: 'cpu', name: 'N/A', vram_mb: 0, recommended_layers: 0 };
}

export async function GetLlamaConfig() {
  if (isWails) {
    const w = await getWailsApp();
    return w.GetLlamaConfig();
  }
  return { binary_path: '', port: 8081, ctx_size: 4096, models_dir: './data/models' };
}

export async function SetLlamaBinaryPath(path) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SetLlamaBinaryPath(path);
  }
}

export async function CheckLlamaInstallation() {
  if (isWails) {
    const w = await getWailsApp();
    return w.CheckLlamaInstallation();
  }
  return false;
}

export async function InstallLlamaServer() {
  if (isWails) {
    const w = await getWailsApp();
    return w.InstallLlamaServer();
  }
}
export async function SendMessageStream(msg) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageStream(msg);
  }
  // Fallback to sync on web for now
  return SendMessage(msg);
}

export async function SendMessageWithImageStream(msg, payload) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageWithImageStream(msg, payload);
  }
  return SendMessageWithImage(msg, payload);
}

export async function SendMessageWithFileStream(msg, payload) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageWithFileStream(msg, payload);
  }
  return SendMessageWithFile(msg, payload);
}
