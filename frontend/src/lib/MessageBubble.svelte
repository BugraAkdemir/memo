<script>
  import { marked } from 'marked';

  export let message;
  let copied = false;

  marked.setOptions({ breaks: true, gfm: true });

  $: html = message.role === 'assistant' && message.content
    ? marked.parse(message.content)
    : '';

  function copy() {
    navigator.clipboard.writeText(message.content);
    copied = true;
    setTimeout(() => copied = false, 1500);
  }
</script>

<div class="row {message.role} fade-in">
  {#if message.role === 'assistant'}
    <div class="av av-bot">⬡</div>
  {/if}

  <div class="body">
    <div class="bubble" class:user={message.role === 'user'} class:bot={message.role === 'assistant'}>
      {#if message.role === 'user'}
        <p>{message.content}</p>
      {:else}
        <div class="md">{@html html}</div>
      {/if}
    </div>
    <div class="meta">
      <span class="ts">{message.timestamp}</span>
      {#if message.role === 'assistant' && message.content}
        <button class="cp" on:click={copy}>
          {copied ? '✓' : '⎘'}
        </button>
      {/if}
    </div>
  </div>

  {#if message.role === 'user'}
    <div class="av av-user">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>
      </svg>
    </div>
  {/if}
</div>

<style>
  .row {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    margin-bottom: 2px;
  }
  .row.user { justify-content: flex-end; }
  .row.assistant { justify-content: flex-start; }
  .av {
    width: 30px;
    height: 30px;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    font-size: 13px;
    font-weight: 700;
    margin-top: 2px;
  }
  .av-bot {
    background: linear-gradient(135deg, var(--neon-purple-dim), var(--neon-blue-dim));
    color: white;
    box-shadow: 0 0 12px var(--glow-purple);
    text-shadow: 0 0 6px rgba(255,255,255,0.5);
  }
  .av-user {
    background: var(--bg-elevated);
    color: var(--text-secondary);
    border: 1px solid var(--border-subtle);
  }
  .body {
    max-width: 78%;
    min-width: 0;
  }
  .bubble {
    padding: 10px 15px;
    border-radius: var(--radius-lg);
    font-size: 14px;
    line-height: 1.65;
    word-wrap: break-word;
  }
  .bubble.user {
    background: linear-gradient(135deg, var(--neon-purple-dim), var(--neon-purple));
    color: white;
    border-bottom-right-radius: 4px;
    box-shadow: 0 2px 16px var(--glow-purple);
  }
  .bubble.bot {
    background: var(--glass-bg);
    border: 1px solid var(--glass-border);
    color: var(--text-primary);
    border-bottom-left-radius: 4px;
  }
  .bubble p { margin: 0; }
  .meta {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 3px;
    padding: 0 4px;
  }
  .ts {
    font-size: 10px;
    color: var(--text-muted);
  }
  .cp {
    font-size: 13px;
    color: var(--text-muted);
    opacity: 0;
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
  }
  .row:hover .cp { opacity: 1; }
  .cp:hover { background: var(--bg-hover); color: var(--neon-blue); }
</style>
