<script>
  import { messages, isStreaming } from '../stores/chat.js';
  import MessageBubble from './MessageBubble.svelte';
  import { tick } from 'svelte';

  let chatEl;
  let prevLen = 0;

  $: if ($messages.length !== prevLen) {
    prevLen = $messages.length;
    scrollToBottom();
  }

  // Also scroll on streaming content updates
  $: if ($messages.length > 0) {
    const last = $messages[$messages.length - 1];
    if (last && last.role === 'assistant') {
      scrollToBottom();
    }
  }

  async function scrollToBottom() {
    await tick();
    if (chatEl) {
      chatEl.scrollTop = chatEl.scrollHeight;
    }
  }
</script>

<div class="chat" bind:this={chatEl}>
  {#if $messages.length === 0}
    <div class="empty fade-in">
      <div class="empty-glow">⬡</div>
      <h2>Cortex</h2>
      <p>Local AI · Persistent Memory · 100% Private</p>
      <div class="hints">
        <div class="hint">💬 Bir soru sor</div>
        <div class="hint">🧠 Her şeyi hatırlarım</div>
        <div class="hint">🔒 Tamamen yerel</div>
      </div>
    </div>
  {:else}
    <div class="msgs">
      {#each $messages as message (message.id)}
        <MessageBubble {message} />
      {/each}
      {#if $isStreaming}
        <div class="typing fade-in">
          <span class="td"></span>
          <span class="td"></span>
          <span class="td"></span>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .chat {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
  }
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    text-align: center;
    gap: 10px;
  }
  .empty-glow {
    font-size: 64px;
    color: var(--neon-purple);
    filter: drop-shadow(0 0 30px var(--glow-purple)) drop-shadow(0 0 60px rgba(179,71,255,0.15));
    animation: neon-breathe 3s ease-in-out infinite;
    margin-bottom: 6px;
  }
  .empty h2 {
    font-size: 32px;
    font-weight: 700;
    color: var(--neon-purple);
    text-shadow: 0 0 20px var(--glow-purple);
    letter-spacing: -0.5px;
  }
  .empty p {
    color: var(--text-muted);
    font-size: 14px;
    margin-bottom: 16px;
  }
  .hints {
    display: flex;
    gap: 10px;
    flex-wrap: wrap;
    justify-content: center;
  }
  .hint {
    padding: 8px 18px;
    border-radius: var(--radius-lg);
    font-size: 13px;
    color: var(--text-secondary);
    background: var(--glass-bg);
    border: 1px solid var(--glass-border);
    transition: all var(--transition-fast);
    cursor: default;
  }
  .hint:hover {
    border-color: var(--neon-purple);
    color: var(--text-primary);
    box-shadow: 0 0 12px var(--glow-purple);
    transform: translateY(-2px);
  }
  .msgs {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-width: 820px;
    margin: 0 auto;
  }
  .typing {
    display: flex;
    gap: 5px;
    padding: 12px 16px;
    margin-left: 44px;
  }
  .td {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--neon-purple);
    animation: typing-dot 1.2s infinite;
  }
  .td:nth-child(2) { animation-delay: 0.15s; }
  .td:nth-child(3) { animation-delay: 0.3s; }
</style>
