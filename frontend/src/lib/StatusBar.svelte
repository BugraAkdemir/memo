<script>
  import { connectionStatus, memoryCount, sidebarOpen, memoryPanelOpen, config } from '../stores/chat.js';

  $: modelName = $connectionStatus.models && $connectionStatus.models.length > 0
    ? $connectionStatus.models[0] : 'Not connected';
</script>

<header class="bar">
  <div class="bar-left">
    <button class="icon-btn" on:click={() => sidebarOpen.update(v => !v)} title="Settings">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <line x1="3" y1="7" x2="21" y2="7"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="17" x2="21" y2="17"/>
      </svg>
    </button>
    <div class="brand">
      <span class="logo">⬡</span>
      <span class="name">{$config.identity.assistant_name || 'Cortex'}</span>
    </div>
  </div>

  <div class="bar-center">
    <div class="conn" class:on={$connectionStatus.connected}>
      <span class="dot"></span>
      <span class="lbl">{$connectionStatus.connected ? modelName : 'Disconnected'}</span>
    </div>
  </div>

  <div class="bar-right">
    <div class="mem-badge" title="Stored memories">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
      </svg>
      <span>{$memoryCount}</span>
    </div>
    <button class="icon-btn" on:click={() => memoryPanelOpen.update(v => !v)} title="Memory Explorer">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
      </svg>
    </button>
  </div>
</header>

<style>
  .bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 16px;
    height: 46px;
    background: rgba(10,10,20,0.85);
    backdrop-filter: blur(16px);
    border-bottom: 1px solid var(--border-subtle);
    z-index: 100;
    flex-shrink: 0;
  }
  .bar-left, .bar-right {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .logo {
    font-size: 20px;
    color: var(--neon-purple);
    filter: drop-shadow(0 0 6px var(--glow-purple));
    animation: neon-breathe 4s ease-in-out infinite;
  }
  .name {
    font-weight: 700;
    font-size: 16px;
    letter-spacing: -0.3px;
    color: var(--neon-purple);
    text-shadow: 0 0 12px var(--glow-purple);
  }
  .conn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 14px;
    border-radius: var(--radius-full);
    font-size: 12px;
    font-weight: 500;
    background: var(--bg-tertiary);
    border: 1px solid var(--border-subtle);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--status-offline);
    box-shadow: 0 0 6px var(--status-offline);
    transition: all 300ms;
  }
  .conn.on .dot {
    background: var(--status-online);
    box-shadow: 0 0 8px var(--status-online);
  }
  .lbl {
    color: var(--text-secondary);
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mem-badge {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 4px 10px;
    border-radius: var(--radius-full);
    font-size: 12px;
    font-weight: 500;
    color: var(--text-muted);
    background: var(--bg-tertiary);
    border: 1px solid var(--border-subtle);
  }
  .icon-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 34px;
    height: 34px;
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
  }
  .icon-btn:hover {
    background: var(--bg-hover);
    color: var(--neon-purple);
  }
</style>
