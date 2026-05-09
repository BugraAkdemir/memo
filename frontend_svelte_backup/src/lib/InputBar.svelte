<script>
  import { isStreaming, addMessage, appendToLastAssistant, memoryCount } from '../stores/chat.js';
  import { SendMessage, SendMessageStream, GetMemoryCount } from '../../wailsjs/go/main/App.js';
  import { EventsOn } from '../../wailsjs/runtime/runtime.js';
  import { onMount } from 'svelte';

  let input = '';
  let ta;
  let streaming = false; // DEFAULT: sync mode (guaranteed to work)

  function resize() {
    if (!ta) return;
    ta.style.height = 'auto';
    ta.style.height = Math.min(ta.scrollHeight, 180) + 'px';
  }

  async function send() {
    const msg = input.trim();
    if (!msg || $isStreaming) return;

    // Clear input FIRST
    input = '';
    if (ta) { ta.style.height = 'auto'; ta.value = ''; }

    // Show user message immediately
    addMessage('user', msg);

    if (streaming) {
      await doStream(msg);
    } else {
      await doSync(msg);
    }

    // Update memory count
    try { memoryCount.set(await GetMemoryCount()); } catch(e) {}
  }

  async function doSync(msg) {
    isStreaming.set(true);
    try {
      console.log('[Cortex] Sending sync:', msg);
      const response = await SendMessage(msg);
      console.log('[Cortex] Got response:', response?.substring(0, 80));
      addMessage('assistant', response || '(empty response)');
    } catch (e) {
      console.error('[Cortex] Error:', e);
      addMessage('assistant', '⚠️ ' + (e?.message || e?.toString() || 'Unknown error'));
    } finally {
      isStreaming.set(false);
    }
  }

  async function doStream(msg) {
    isStreaming.set(true);
    addMessage('assistant', '');

    const offChunk = EventsOn('stream:chunk', (data) => {
      if (!data) return;
      if (data.error) {
        appendToLastAssistant('⚠️ ' + data.error);
        isStreaming.set(false);
      }
      if (data.content) {
        appendToLastAssistant(data.content);
      }
      if (data.done) {
        isStreaming.set(false);
      }
    });

    try {
      console.log('[Cortex] Sending stream:', msg);
      await SendMessageStream(msg);
      console.log('[Cortex] Stream complete');
    } catch (e) {
      console.error('[Cortex] Stream error:', e);
      appendToLastAssistant('⚠️ ' + (e?.message || e?.toString() || 'Stream error'));
    } finally {
      isStreaming.set(false);
      setTimeout(() => { if (offChunk) offChunk(); }, 500);
    }
  }

  function onKey(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }

  onMount(() => { if (ta) ta.focus(); });
</script>

<div class="bar">
  <div class="box" class:active={$isStreaming}>
    <textarea
      bind:this={ta}
      bind:value={input}
      on:input={resize}
      on:keydown={onKey}
      placeholder="Mesajını yaz..."
      rows="1"
      disabled={$isStreaming}
    ></textarea>
    <div class="actions">
      <button
        class="mode"
        class:on={streaming}
        on:click={() => streaming = !streaming}
        title={streaming ? 'Stream mode (live typing)' : 'Sync mode (full response)'}
        disabled={$isStreaming}
      >
        {streaming ? '⚡' : '◼'}
      </button>
      <button class="send" on:click={send} disabled={!input.trim() || $isStreaming}>
        {#if $isStreaming}
          <div class="spinner"></div>
        {:else}
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 2L11 13"/><path d="M22 2L15 22L11 13L2 9L22 2"/>
          </svg>
        {/if}
      </button>
    </div>
  </div>
  <div class="hint">
    Enter gönder · Shift+Enter yeni satır · <span class="ml">{streaming ? '⚡ Stream' : '◼ Sync'}</span>
  </div>
</div>

<style>
  .bar {
    padding: 0 24px 14px;
    max-width: 868px;
    margin: 0 auto;
    width: 100%;
  }
  .box {
    display: flex;
    align-items: flex-end;
    gap: 6px;
    padding: 8px 10px 8px 16px;
    border-radius: var(--radius-xl);
    background: rgba(12,12,26,0.85);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(179,71,255,0.12);
    transition: border-color 200ms, box-shadow 200ms;
  }
  .box:focus-within {
    border-color: var(--neon-purple);
    box-shadow: 0 0 0 2px rgba(179,71,255,0.2), 0 0 30px rgba(179,71,255,0.06);
  }
  .box.active {
    border-color: var(--neon-blue);
    box-shadow: 0 0 0 2px rgba(0,212,255,0.2);
  }
  textarea {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-primary);
    font-size: 14px;
    font-family: var(--font-sans);
    line-height: 1.5;
    resize: none;
    min-height: 22px;
    max-height: 180px;
    padding: 5px 0;
    outline: none;
  }
  textarea::placeholder { color: var(--text-muted); }
  textarea:disabled { opacity: 0.4; }
  .actions { display:flex; align-items:center; gap:4px; padding-bottom:1px; }
  .mode {
    width:32px; height:32px; border-radius:var(--radius-sm);
    font-size:14px; color:var(--text-muted);
    display:flex; align-items:center; justify-content:center;
  }
  .mode:hover { background:var(--bg-hover); }
  .mode.on { color:var(--neon-blue); }
  .send {
    display:flex; align-items:center; justify-content:center;
    width:38px; height:38px; border-radius:var(--radius-md);
    background:linear-gradient(135deg,#7b2ff2,#b347ff);
    color:white; flex-shrink:0;
    box-shadow:0 2px 16px rgba(179,71,255,0.3);
  }
  .send:hover:not(:disabled) {
    transform:scale(1.05);
    box-shadow:0 4px 24px rgba(179,71,255,0.45);
  }
  .send:disabled { opacity:0.2; transform:none; box-shadow:none; }
  .spinner {
    width:16px; height:16px;
    border:2px solid rgba(255,255,255,0.25);
    border-top-color:white;
    border-radius:50%;
    animation:spin 0.5s linear infinite;
  }
  @keyframes spin { to { transform:rotate(360deg); } }
  .hint {
    text-align:center;
    margin-top:6px;
    font-size:11px;
    color:var(--text-muted);
    opacity:0.4;
  }
  .ml { color:var(--neon-blue); }
</style>
