<script>
  import { onMount, onDestroy } from 'svelte';
  import { CheckAuth, StartSyncAuth, TriggerSync } from './api-bridge.js';
  import {
    initCloudSyncStore,
    setCloudSyncAuthenticated,
    syncAuthenticated,
    syncError,
    syncStatus,
    teardownCloudSyncStore
  } from '../stores/cloudSync.js';

  // Possible states: 'idle' | 'archiving' | 'encrypting' | 'uploading' | 'pruning' | 'done' | 'error' | 'busy'
  let doneTimer = null;

  const LABELS = {
    idle:       'Cloud Sync',
    archiving:  'Archiving…',
    encrypting: 'Encrypting…',
    uploading:  'Uploading…',
    pruning:    'Cleaning up…',
    done:       'Backed up',
    error:      'Sync error',
    busy:       'Busy…',
  };

  onMount(async () => {
    await initCloudSyncStore();
  });

  onDestroy(() => {
    teardownCloudSyncStore();
    clearTimeout(doneTimer);
  });

  async function handleConnect() {
    await StartSyncAuth();
    // Poll until auth completes (loopback server handles the redirect)
    const poll = setInterval(async () => {
      const ok = await CheckAuth().catch(() => false);
      if (ok) {
        setCloudSyncAuthenticated(true);
        clearInterval(poll);
      }
    }, 1500);
    // Stop polling after 3 minutes
    setTimeout(() => clearInterval(poll), 180_000);
  }

  function handleManualSync() {
    TriggerSync();
  }

  $: isActive = ['archiving', 'encrypting', 'uploading', 'pruning'].includes($syncStatus);
  $: label = LABELS[$syncStatus] ?? $syncStatus;
  $: {
    clearTimeout(doneTimer);
    if ($syncStatus === 'done') {
      doneTimer = setTimeout(() => syncStatus.set('idle'), 4000);
    }
    if ($syncStatus === 'error') {
      doneTimer = setTimeout(() => {
        syncStatus.set('idle');
        syncError.set('');
      }, 6000);
    }
  }
</script>

<div class="sync-wrap" title={$syncError || label}>
  {#if !$syncAuthenticated}
    <button class="sync-btn connect" on:click={handleConnect}>
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M18 8h1a4 4 0 0 1 0 8h-1"/>
        <path d="M2 8h16v9a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4V8z"/>
        <line x1="6" y1="1" x2="6" y2="4"/>
        <line x1="10" y1="1" x2="10" y2="4"/>
        <line x1="14" y1="1" x2="14" y2="4"/>
      </svg>
      Connect Drive
    </button>
  {:else}
    <button
      class="sync-btn"
      class:active={isActive}
      class:done={$syncStatus === 'done'}
      class:error={$syncStatus === 'error'}
      on:click={handleManualSync}
      disabled={isActive}
    >
      <!-- Cloud icon -->
      <svg class="icon" class:spin={isActive} width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        {#if isActive}
          <!-- Spinner circle -->
          <circle cx="12" cy="12" r="9" stroke-dasharray="28 56" stroke-linecap="round"/>
        {:else if $syncStatus === 'done'}
          <!-- Checkmark -->
          <polyline points="20 6 9 17 4 12"/>
        {:else if $syncStatus === 'error'}
          <!-- Warning -->
          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/>
          <line x1="12" y1="17" x2="12.01" y2="17"/>
        {:else}
          <!-- Cloud with up arrow -->
          <polyline points="16 16 12 12 8 16"/>
          <line x1="12" y1="12" x2="12" y2="21"/>
          <path d="M20.39 18.39A5 5 0 0 0 18 9h-1.26A8 8 0 1 0 3 16.3"/>
        {/if}
      </svg>
      {label}
    </button>
  {/if}
</div>

<style>
  .sync-wrap {
    display: flex;
    align-items: center;
  }

  .sync-btn {
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
    cursor: pointer;
    transition: color 200ms, border-color 200ms, background 200ms;
    white-space: nowrap;
  }

  .sync-btn:hover:not(:disabled) {
    color: var(--text-secondary);
    border-color: var(--border-default, #444);
  }

  .sync-btn:disabled {
    cursor: default;
    opacity: 0.8;
  }

  .sync-btn.connect {
    color: var(--neon-purple, #a78bfa);
    border-color: var(--neon-purple, #a78bfa);
  }

  .sync-btn.done {
    color: var(--status-online, #4ade80);
    border-color: var(--status-online, #4ade80);
  }

  .sync-btn.error {
    color: #f87171;
    border-color: #f87171;
  }

  .sync-btn.active {
    color: var(--text-secondary);
  }

  .icon.spin {
    animation: spin 1s linear infinite;
    transform-origin: center;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to   { transform: rotate(360deg); }
  }
</style>
