import { writable } from 'svelte/store';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js';
import { CheckAuth } from '../lib/api-bridge.js';

export const syncAuthenticated = writable(false);
export const syncStatus = writable('idle');
export const syncError = writable('');

let bound = false;

export async function initCloudSyncStore() {
  if (bound) return;
  bound = true;

  syncAuthenticated.set(await CheckAuth().catch(() => false));

  EventsOn('sync:status', (payload) => {
    const status = typeof payload === 'string' ? payload : payload?.status;
    if (!status) return;
    syncStatus.set(status);
    if (status !== 'error') {
      syncError.set('');
    }
  });

  EventsOn('sync:error', (message) => {
    syncStatus.set('error');
    syncError.set(String(message || 'Unknown sync error'));
  });
}

export function setCloudSyncAuthenticated(value) {
  syncAuthenticated.set(Boolean(value));
}

export function teardownCloudSyncStore() {
  if (!bound) return;
  EventsOff('sync:status');
  EventsOff('sync:error');
  bound = false;
}
