<script>
  import { config, sidebarOpen } from '../stores/chat.js';
  import { UpdateIdentity, GetAvailableStyles } from '../../wailsjs/go/main/App.js';
  import { onMount } from 'svelte';

  let styles = ['casual', 'formal', 'technical', 'creative'];
  let userName = '';
  let assistantName = '';
  let selectedStyle = '';
  let saving = false;
  let saveMsg = '';

  onMount(async () => {
    userName = $config.identity.user_name || 'User';
    assistantName = $config.identity.assistant_name || 'Cortex';
    selectedStyle = $config.identity.style || 'casual';
    try {
      const s = await GetAvailableStyles();
      if (s && s.length) styles = s;
    } catch(e) {}
  });

  async function save() {
    saving = true;
    try {
      await UpdateIdentity(userName, assistantName, selectedStyle);
      config.update(c => ({ ...c, identity: { ...c.identity, user_name: userName, assistant_name: assistantName, style: selectedStyle } }));
      saveMsg = '✓';
      setTimeout(() => saveMsg = '', 1500);
    } catch(e) {
      saveMsg = '✗ ' + e;
    }
    saving = false;
  }
</script>

<aside class="side fade-in">
  <div class="side-head">
    <h3>Settings</h3>
    <button class="x" on:click={() => sidebarOpen.set(false)}>✕</button>
  </div>

  <div class="side-body">
    <div class="sec">
      <div class="sec-title">🎭 Identity</div>
      <label class="field">
        <span>Your Name</span>
        <input type="text" bind:value={userName} />
      </label>
      <label class="field">
        <span>Assistant Name</span>
        <input type="text" bind:value={assistantName} />
      </label>
      <label class="field">
        <span>Style</span>
        <select bind:value={selectedStyle}>
          {#each styles as s}
            <option value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>
          {/each}
        </select>
      </label>
    </div>

    <div class="sec">
      <div class="sec-title">⚡ Connection</div>
      <div class="info"><span>Endpoint</span><span class="val">{$config.api.base_url}</span></div>
      <div class="info"><span>Embedding</span><span class="val">{$config.api.embedding_model}</span></div>
    </div>
  </div>

  <div class="side-foot">
    <button class="save-btn" on:click={save} disabled={saving}>
      {saving ? '...' : 'Save'}
    </button>
    {#if saveMsg}<span class="sm">{saveMsg}</span>{/if}
  </div>
</aside>

<style>
  .side {
    width: 280px;
    height: 100%;
    display: flex;
    flex-direction: column;
    background: rgba(10,10,20,0.9);
    backdrop-filter: blur(16px);
    border-right: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .side-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 18px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .side-head h3 { font-size: 14px; font-weight: 600; }
  .x {
    color: var(--text-muted);
    font-size: 16px;
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-sm);
  }
  .x:hover { background: var(--bg-hover); color: var(--text-primary); }
  .side-body { flex: 1; overflow-y: auto; padding: 18px; }
  .sec { margin-bottom: 24px; }
  .sec-title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: var(--text-muted);
    margin-bottom: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 12px;
  }
  .field span { font-size: 12px; color: var(--text-secondary); }
  .field input, .field select { width: 100%; font-size: 13px; padding: 8px 12px; }
  .info {
    display: flex;
    justify-content: space-between;
    padding: 7px 0;
    border-bottom: 1px solid var(--border-subtle);
    font-size: 12px;
    color: var(--text-muted);
  }
  .val { color: var(--text-secondary); font-family: var(--font-mono); font-size: 11px; }
  .side-foot {
    padding: 14px 18px;
    border-top: 1px solid var(--border-subtle);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .save-btn {
    flex: 1;
    padding: 9px;
    border-radius: var(--radius-md);
    background: linear-gradient(135deg, var(--neon-purple-dim), var(--neon-purple));
    color: white;
    font-weight: 600;
    font-size: 13px;
    box-shadow: 0 2px 12px var(--glow-purple);
  }
  .save-btn:hover:not(:disabled) {
    transform: translateY(-1px);
    box-shadow: 0 4px 20px var(--glow-purple);
  }
  .sm { font-size: 12px; color: var(--status-online); }
</style>
