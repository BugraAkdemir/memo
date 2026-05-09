<script>
  import { currentMemories, memoryPanelOpen } from '../stores/chat.js';
  import { GetMemoryContext } from '../../wailsjs/go/main/App.js';

  let q = '';
  let busy = false;

  async function search() {
    if (!q.trim()) return;
    busy = true;
    try {
      const r = await GetMemoryContext(q);
      currentMemories.set(r || []);
    } catch(e) {
      currentMemories.set([]);
    }
    busy = false;
  }
</script>

<aside class="mp fade-in">
  <div class="mp-head">
    <h3>🧠 Memory</h3>
    <button class="x" on:click={() => memoryPanelOpen.set(false)}>✕</button>
  </div>

  <div class="mp-search">
    <input type="text" bind:value={q} on:keydown={e => e.key === 'Enter' && search()} placeholder="Search memories..." />
    <button class="sbtn" on:click={search} disabled={busy || !q.trim()}>
      {busy ? '...' : '→'}
    </button>
  </div>

  <div class="mp-results">
    {#if $currentMemories.length === 0}
      <div class="mp-empty">
        <p>Search past conversations</p>
      </div>
    {:else}
      {#each $currentMemories as m, i}
        <div class="mcard fade-in" style="animation-delay:{i*60}ms">
          <div class="mcard-top">
            <span class="score">{(m.similarity * 100).toFixed(0)}%</span>
            <span class="mtime">{m.timestamp || ''}</span>
          </div>
          <div class="mtxt">{m.content}</div>
        </div>
      {/each}
    {/if}
  </div>
</aside>

<style>
  .mp {
    width: 320px;
    height: 100%;
    display: flex;
    flex-direction: column;
    background: rgba(10,10,20,0.9);
    backdrop-filter: blur(16px);
    border-left: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .mp-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .mp-head h3 { font-size: 14px; font-weight: 600; }
  .x {
    color: var(--text-muted); font-size: 16px;
    width: 28px; height: 28px;
    display: flex; align-items: center; justify-content: center;
    border-radius: var(--radius-sm);
  }
  .x:hover { background: var(--bg-hover); color: var(--text-primary); }
  .mp-search {
    display: flex;
    gap: 6px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .mp-search input { flex: 1; font-size: 13px; padding: 8px 12px; }
  .sbtn {
    padding: 8px 14px;
    border-radius: var(--radius-sm);
    background: var(--neon-blue-dim);
    color: white;
    font-weight: 700;
    font-size: 14px;
  }
  .sbtn:disabled { opacity: 0.3; }
  .mp-results { flex: 1; overflow-y: auto; padding: 12px; }
  .mp-empty {
    display: flex; align-items: center; justify-content: center;
    height: 160px; text-align: center;
  }
  .mp-empty p { font-size: 13px; color: var(--text-muted); }
  .mcard {
    padding: 10px;
    border-radius: var(--radius-md);
    margin-bottom: 8px;
    background: var(--glass-bg);
    border: 1px solid var(--border-subtle);
  }
  .mcard:hover { border-color: var(--glass-border-hover); }
  .mcard-top {
    display: flex;
    justify-content: space-between;
    margin-bottom: 6px;
  }
  .score {
    padding: 1px 8px;
    border-radius: var(--radius-full);
    font-size: 11px;
    font-weight: 600;
    background: rgba(0,212,255,0.15);
    color: var(--neon-blue);
  }
  .mtime { font-size: 10px; color: var(--text-muted); }
  .mtxt {
    font-size: 12px;
    line-height: 1.5;
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-break: break-word;
  }
</style>
