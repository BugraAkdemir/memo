import { writable } from 'svelte/store';

export const messages = writable([]);
export const isStreaming = writable(false);
export const connectionStatus = writable({ connected: false, models: [] });
export const memoryCount = writable(0);
export const sidebarOpen = writable(false);
export const memoryPanelOpen = writable(false);
export const currentMemories = writable([]);

export const config = writable({
  identity: { user_name: 'User', assistant_name: 'Cortex', style: 'casual', system_role: '' },
  api: { base_url: 'http://localhost:1234/v1', embedding_model: 'nomic-embed-text-v1.5', timeout_seconds: 120 },
  memory: { persist_dir: './data/memory', top_k: 5, min_similarity: 0.3 }
});

let msgCounter = 0;

export function addMessage(role, content) {
  msgCounter++;
  messages.update(msgs => [...msgs, {
    id: msgCounter,
    role,
    content,
    timestamp: new Date().toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })
  }]);
}

export function appendToLastAssistant(text) {
  messages.update(msgs => {
    if (msgs.length === 0) return msgs;
    const last = msgs[msgs.length - 1];
    if (last.role !== 'assistant') return msgs;
    const updated = [...msgs];
    updated[updated.length - 1] = { ...last, content: last.content + text };
    return updated;
  });
}

export function clearMessages() {
  messages.set([]);
  msgCounter = 0;
}
