import { create } from 'zustand';
import { auth } from '../api';
import { deriveKEK, unwrapMasterKey } from '../utils/e2ee';
import type { E2EEStatus } from '../api';

interface E2EEState {
  /** Whether the account has E2EE enabled (server-side flag). */
  enabled: boolean;
  /** In-memory unlocked master key. Never persisted. */
  masterKey: CryptoKey | null;
  statusLoaded: boolean;
  /** Load status from server (call on login). */
  loadStatus: () => Promise<void>;
  /** Unlock the master key by deriving the KEK from the passphrase. */
  unlock: (passphrase: string) => Promise<boolean>;
  /** Forget the master key (lock). */
  lock: () => void;
  /** Set enabled state after setup/disable. */
  setEnabled: (enabled: boolean) => void;
}

export const useE2EEStore = create<E2EEState>((set, get) => ({
  enabled: false,
  masterKey: null,
  statusLoaded: false,

  loadStatus: async () => {
    try {
      const status: E2EEStatus = await auth.e2eeStatus();
      set({ enabled: status.enabled, statusLoaded: true });
    } catch {
      set({ enabled: false, statusLoaded: true });
    }
  },

  unlock: async (passphrase: string) => {
    const status: E2EEStatus = await auth.e2eeStatus();
    if (!status.enabled || !status.salt || !status.wrapped_key) return false;
    try {
      const kek = await deriveKEK(passphrase, status.salt);
      const masterKey = await unwrapMasterKey(status.wrapped_key, kek);
      set({ masterKey, enabled: true });
      return true;
    } catch {
      return false; // wrong passphrase or corrupted wrapped key
    }
  },

  lock: () => set({ masterKey: null }),

  setEnabled: (enabled: boolean) => set({ enabled, masterKey: enabled ? get().masterKey : null }),
}));
