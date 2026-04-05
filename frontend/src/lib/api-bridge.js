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

export async function SendMessageWithImage(msg, imagePath) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageWithImage(msg, imagePath);
  }
  // Image upload not supported on web - send text only
  const data = await restPost('/send', { message: msg });
  return data.reply;
}

export async function SendMessageWithFile(msg, filePath) {
  if (isWails) {
    const w = await getWailsApp();
    return w.SendMessageWithFile(msg, filePath);
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

export async function StartRecording() {
  if (isWails) {
    const w = await getWailsApp();
    return w.StartRecording();
  }
  throw new Error('Voice recording is not available on web');
}

export async function StopRecordingAndTranscribe() {
  if (isWails) {
    const w = await getWailsApp();
    return w.StopRecordingAndTranscribe();
  }
  throw new Error('Voice recording is not available on web');
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

// Utility: check if running in Wails
export function isWailsEnvironment() {
  return isWails;
}
